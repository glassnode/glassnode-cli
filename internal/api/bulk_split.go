package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

// Bulk requests are limited to a time range that depends on the resolution;
// see https://docs.glassnode.com/basic-api/bulk-metrics#timerange-constraints.
const (
	bulkWindowFine    = 10 * 24 * time.Hour // 10m and 1h
	bulkWindowDaily   = 31 * 24 * time.Hour // 24h, the default resolution
	bulkWindowCoarse  = 93 * 24 * time.Hour // 1w and 1month
	defaultResolution = "24h"

	// When the API still rejects a window as too large, the window is halved
	// this many times before giving up.
	maxWindowShrinks = 3
)

// intervalDuration is the length of one data interval at the resolution; a
// window shorter than it returns no points, so halving stops there.
func intervalDuration(interval string) time.Duration {
	switch interval {
	case "10m":
		return 10 * time.Minute
	case "1h":
		return time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1month":
		return 31 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// intervalStart returns the start of the interval containing t, in UTC: weeks
// start on Monday, months on the 1st. The API rounds s and u down this way and
// measures the range between the rounded bounds.
func intervalStart(t time.Time, interval string) time.Time {
	t = t.UTC()
	switch interval {
	case "10m":
		return t.Truncate(10 * time.Minute)
	case "1h":
		return t.Truncate(time.Hour)
	case "1w":
		midnight := t.Truncate(24 * time.Hour)
		return midnight.AddDate(0, 0, -(int(midnight.Weekday())+6)%7)
	case "1month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return t.Truncate(24 * time.Hour)
	}
}

// knownInterval reports whether the API offers the resolution; the window
// helpers fall back to 24h, so an unknown one must be refused before planning.
func knownInterval(interval string) bool {
	switch interval {
	case "10m", "1h", "24h", "1w", "1month":
		return true
	}
	return false
}

// BulkWindow returns the longest time range the API accepts in one bulk
// request at the given resolution.
func BulkWindow(interval string) time.Duration {
	switch interval {
	case "10m", "1h":
		return bulkWindowFine
	case "1w", "1month":
		return bulkWindowCoarse
	default:
		return bulkWindowDaily
	}
}

// BulkPlan is the time range of a bulk request and the window it is fetched in.
type BulkPlan struct {
	Since, Until time.Time
	Interval     string
	Window       time.Duration
}

// Window is one request's time range: Since inclusive, Until exclusive.
type Window struct{ Since, Until time.Time }

// PlanBulk reads the s, u and i parameters of a bulk request. s is required
// by the API; u defaults to now and is capped at now, since the API returns no
// future points. now is truncated to seconds, the API's resolution, so that a
// range ending now is never a fraction of a second over the window.
func PlanBulk(params map[string]string, now time.Time) (BulkPlan, error) {
	plan := BulkPlan{Interval: params["i"], Until: now.Truncate(time.Second)}
	if plan.Interval == "" {
		plan.Interval = defaultResolution
	}
	if !knownInterval(plan.Interval) {
		return plan, fmt.Errorf("unknown --interval %q; bulk metrics accept 10m, 1h, 24h, 1w or 1month", plan.Interval)
	}
	plan.Window = BulkWindow(plan.Interval)
	raw, ok := params["s"]
	if !ok {
		return plan, fmt.Errorf("bulk metrics require --since (at most %s per request at %s; --split fetches a longer range in several requests)",
			plan.WindowDays(), plan.Interval)
	}
	since, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return plan, fmt.Errorf("parameter s: expected Unix seconds, got %q", raw)
	}
	// The API counts the range from the start of the interval containing s,
	// so the plan does too: at 1w, --since 93d reaches back up to 99 days.
	plan.Since = intervalStart(time.Unix(since, 0), plan.Interval)
	// boundedByNow: the range ends at now, either because --until was not
	// given or because it lay in the future; the error then talks about now,
	// not about an --until the user never saw.
	boundedByNow := true
	if raw, ok := params["u"]; ok {
		until, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return plan, fmt.Errorf("parameter u: expected Unix seconds, got %q", raw)
		}
		if t := time.Unix(until, 0).UTC(); t.Before(plan.Until) {
			plan.Until = t
			boundedByNow = false
		}
	}
	plan.Until = plan.Until.UTC()
	if !plan.Since.Before(plan.Until) {
		if boundedByNow {
			return plan, fmt.Errorf("--since (%s) is not in the past", plan.Since.Format(time.RFC3339))
		}
		return plan, fmt.Errorf("--since (%s) must be before --until (%s)", plan.Since.Format(time.RFC3339), plan.Until.Format(time.RFC3339))
	}
	return plan, nil
}

// Windows splits the range into consecutive windows the API accepts: from the
// start of the interval containing s to the start of the one containing u is
// at most plan.Window. Inner bounds are interval starts, and each window's
// Until is the next window's Since: the API includes the interval containing s
// and only intervals that end by u, so consecutive windows neither overlap nor
// leave gaps.
func (plan BulkPlan) Windows() []Window {
	var windows []Window
	for since := plan.Since; since.Before(plan.Until); {
		limit := since.Add(plan.Window)
		until := plan.Until
		if intervalStart(until, plan.Interval).After(limit) {
			until = intervalStart(limit, plan.Interval)
			if !until.After(since) {
				// A window shorter than one interval: no bound to round to.
				until = limit
			}
		}
		windows = append(windows, Window{since, until})
		since = until
	}
	return windows
}

// WindowDays is the window's length as shown to the user, such as "31 days".
func (plan BulkPlan) WindowDays() string { return days(plan.Window) }

// RangeError explains why the range needs --split.
func (plan BulkPlan) RangeError() error {
	return fmt.Errorf("range %s..%s exceeds the API limit of %s per bulk request at %s; shorten it or add --split to fetch it in %d requests",
		formatBound(plan.Since), formatBound(plan.Until), plan.WindowDays(), plan.Interval, len(plan.Windows()))
}

// Describe summarises the split for a warning to the user.
func (plan BulkPlan) Describe() string {
	return fmt.Sprintf("splitting %s..%s into %d requests of up to %s (%s); the result is assembled in memory",
		formatBound(plan.Since), formatBound(plan.Until), len(plan.Windows()), plan.WindowDays(), plan.Interval)
}

// WindowParams returns params with s and u set to the window's bounds.
func WindowParams(params map[string]string, w Window) map[string]string {
	out := make(map[string]string, len(params)+2)
	for key, value := range params {
		out[key] = value
	}
	out["s"] = strconv.FormatInt(w.Since.Unix(), 10)
	out["u"] = strconv.FormatInt(w.Until.Unix(), 10)
	return out
}

// GetMetricBulkSplit fetches a bulk metric window by window and merges the
// points. If the API rejects a window as too large, the window is halved for
// the rest of the range, up to maxWindowShrinks times. Any other failure
// aborts the whole fetch: a partial result would look complete to a script.
func (c *Client) GetMetricBulkSplit(ctx context.Context, path string, params map[string]string, repeated map[string][]string, plan BulkPlan, progress io.Writer) (*BulkResponse, error) {
	if progress == nil {
		progress = io.Discard
	}
	merged := &BulkResponse{}
	// Adjacent windows share no timestamp (the API rounds s down and includes
	// only intervals ending by u), so index is a guard: should a timestamp
	// repeat, the later response, which is at least as fresh, replaces it.
	index := map[int64]int{}
	shrinks, fetched := 0, 0
	since := plan.Since
	for since.Before(plan.Until) {
		windows := BulkPlan{Since: since, Until: plan.Until, Interval: plan.Interval, Window: plan.Window}.Windows()
		w := windows[0]
		// One line per window: each goes through the SDK's retries and may
		// wait on a rate limit, so a long split fetch should not fall silent.
		fmt.Fprintf(progress, "window %d/%d %s..%s\n", fetched+1, fetched+len(windows), formatBound(w.Since), formatBound(w.Until))
		resp, err := c.GetMetricBulk(ctx, path, WindowParams(params, w), repeated)
		if err != nil {
			// Halve the range the API actually rejected: near the end of the
			// range it is shorter than plan.Window, and halving the nominal
			// window could repeat the same request. A window shorter than one
			// interval would return no points and turn the rest of the fetch
			// into silent gaps, so halving stops there.
			if halved := w.Until.Sub(w.Since) / 2; IsRangeTooLarge(err) && shrinks < maxWindowShrinks && halved >= intervalDuration(plan.Interval) {
				shrinks++
				plan.Window = halved
				continue
			}
			// Nothing has been printed, so the whole command is what to retry.
			// The total is an estimate: a halved window changes how many remain.
			remedy := "retry the command"
			if IsRangeTooLarge(err) {
				remedy = "shorten the range or use a coarser --interval"
			}
			return nil, fmt.Errorf("window %d of about %d (%s..%s): %w; the fetch is aborted and no data was written (%d windows had succeeded), %s",
				fetched+1, fetched+len(windows), formatBound(w.Since), formatBound(w.Until), err, fetched, remedy)
		}
		fetched++
		for _, point := range resp.Data {
			if i, ok := index[point.T]; ok {
				merged.Data[i] = point
				continue
			}
			index[point.T] = len(merged.Data)
			merged.Data = append(merged.Data, point)
		}
		since = w.Until
	}
	if merged.Data == nil {
		merged.Data = []BulkDataPoint{}
	}
	return merged, nil
}

// IsRangeTooLarge reports the API's rejection of a window as too long ("too
// large time range (...) requested"). Other 400s, including other complaints
// about the range, are not treated as a reason to halve or to suggest --split:
// that would spend requests and replace the API's message with one about
// window size.
func IsRangeTooLarge(err error) bool {
	var apiErr *glassnode.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 400 && strings.Contains(strings.ToLower(apiErr.Detail), "too large")
}

// formatBound shows a window bound as a date when it is at midnight UTC and
// as an RFC 3339 timestamp otherwise, both of which --since accepts, so a
// retry of an hourly or 10-minute fetch resumes exactly at the bound.
func formatBound(t time.Time) string {
	t = t.UTC()
	if t.Equal(t.Truncate(24 * time.Hour)) {
		return t.Format(time.DateOnly)
	}
	return t.Format(time.RFC3339)
}

func days(d time.Duration) string {
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}
