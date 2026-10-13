package api

import (
	"context"
	"errors"
	"fmt"
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
	plan.Window = BulkWindow(plan.Interval)
	raw, ok := params["s"]
	if !ok {
		return plan, fmt.Errorf("bulk metrics require --since (at most %s per request at %s; --split fetches a longer range in several requests)",
			days(plan.Window), plan.Interval)
	}
	since, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return plan, fmt.Errorf("parameter s: expected Unix seconds, got %q", raw)
	}
	plan.Since = time.Unix(since, 0).UTC()
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

// Windows splits the range into consecutive windows of at most plan.Window.
// Each window's Until is the next window's Since: the API includes the
// interval containing s and only intervals that end by u, so consecutive
// windows neither overlap nor leave gaps.
func (plan BulkPlan) Windows() []Window {
	var windows []Window
	for since := plan.Since; since.Before(plan.Until); since = since.Add(plan.Window) {
		until := since.Add(plan.Window)
		if until.After(plan.Until) {
			until = plan.Until
		}
		windows = append(windows, Window{since, until})
	}
	return windows
}

// RangeError explains why the range needs --split.
func (plan BulkPlan) RangeError() error {
	return fmt.Errorf("range %s..%s exceeds the API limit of %s per bulk request at %s; shorten it or add --split to fetch it in %d requests",
		formatBound(plan.Since), formatBound(plan.Until), days(plan.Window), plan.Interval, len(plan.Windows()))
}

// Describe summarises the split for a warning to the user.
func (plan BulkPlan) Describe() string {
	return fmt.Sprintf("splitting %s..%s into %d requests of up to %s (%s); the result is assembled in memory",
		formatBound(plan.Since), formatBound(plan.Until), len(plan.Windows()), days(plan.Window), plan.Interval)
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
func (c *Client) GetMetricBulkSplit(ctx context.Context, path string, params map[string]string, repeated map[string][]string, plan BulkPlan) (*BulkResponse, error) {
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
		resp, err := c.GetMetricBulk(ctx, path, WindowParams(params, w), repeated)
		if err != nil {
			// Halve the range the API actually rejected: near the end of the
			// range it is shorter than plan.Window, and halving the nominal
			// window could repeat the same request. A window shorter than one
			// interval would return no points and turn the rest of the fetch
			// into silent gaps, so halving stops there.
			if halved := w.Until.Sub(w.Since) / 2; isRangeTooLarge(err) && shrinks < maxWindowShrinks && halved >= intervalDuration(plan.Interval) {
				shrinks++
				plan.Window = halved
				continue
			}
			// The total is an estimate: a halved window changes how many remain.
			return nil, fmt.Errorf("window %d of about %d (%s..%s): %w; the fetch is aborted, retry it with --since %s",
				fetched+1, fetched+len(windows), formatBound(w.Since), formatBound(w.Until), err, formatBound(w.Since))
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

// isRangeTooLarge reports the API's rejection of a window as too long. Other
// 400s (an invalid asset, an unsupported parameter combination) are not
// treated as a reason to halve: that would spend requests and replace the
// API's message with one about window size.
func isRangeTooLarge(err error) bool {
	var apiErr *glassnode.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 400 && strings.Contains(strings.ToLower(apiErr.Detail), "time range")
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
