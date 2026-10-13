package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

var (
	day       = 24 * time.Hour
	planNow   = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	planSince = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func unix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

func TestPlanBulk(t *testing.T) {
	t.Run("requires since", func(t *testing.T) {
		_, err := PlanBulk(map[string]string{}, planNow)
		if err == nil || !strings.Contains(err.Error(), "--since") || !strings.Contains(err.Error(), "31 days") || !strings.Contains(err.Error(), "--split fetches") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("defaults and window by resolution", func(t *testing.T) {
		for interval, want := range map[string]time.Duration{"": 31 * day, "24h": 31 * day, "10m": 10 * day, "1h": 10 * day, "1w": 93 * day, "1month": 93 * day} {
			params := map[string]string{"s": unix(planSince)}
			if interval != "" {
				params["i"] = interval
			}
			plan, err := PlanBulk(params, planNow)
			if err != nil || plan.Window != want || !plan.Until.Equal(planNow) {
				t.Errorf("%q: plan=%+v err=%v", interval, plan, err)
			}
		}
	})
	t.Run("a range exactly one window long is one request", func(t *testing.T) {
		for interval, window := range map[string]time.Duration{"24h": 31 * day, "1h": 10 * day, "1month": 93 * day} {
			now := planNow.Add(123456789 * time.Nanosecond) // a clock read after --since was parsed
			plan, err := PlanBulk(map[string]string{"s": unix(planNow.Add(-window)), "i": interval}, now)
			if err != nil || len(plan.Windows()) != 1 || !plan.Until.Equal(planNow) {
				t.Errorf("%s: windows=%v err=%v, want one window ending at %v", interval, plan.Windows(), err, planNow)
			}
		}
	})
	t.Run("until in the future is capped at now", func(t *testing.T) {
		plan, err := PlanBulk(map[string]string{"s": unix(planSince), "u": unix(planNow.Add(400 * day))}, planNow)
		if err != nil || !plan.Until.Equal(planNow) {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
	})
	t.Run("since must precede until", func(t *testing.T) {
		if _, err := PlanBulk(map[string]string{"s": unix(planNow), "u": unix(planSince)}, planNow); err == nil || !strings.Contains(err.Error(), "must be before --until") {
			t.Fatalf("since >= until: %v", err)
		}
		// Without --until the bound is now, and the message must not name --until.
		_, err := PlanBulk(map[string]string{"s": unix(planNow.Add(day))}, planNow)
		if err == nil || !strings.Contains(err.Error(), "is not in the past") || strings.Contains(err.Error(), "--until") {
			t.Fatalf("since in the future: %v", err)
		}
		// A future --until is capped at now; the error must not report the
		// capped value as if the user had given it.
		_, err = PlanBulk(map[string]string{"s": unix(planNow.Add(day)), "u": unix(planNow.Add(2 * day))}, planNow)
		if err == nil || !strings.Contains(err.Error(), "is not in the past") || strings.Contains(err.Error(), "--until") {
			t.Fatalf("since and until in the future: %v", err)
		}
	})
}

// When the rejected window is the shorter tail of the range, halving the
// nominal window would repeat the identical request; the rejected range
// itself is halved instead.
func TestGetMetricBulkSplitHalvesTheRejectedRange(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(12 * day), Interval: "24h", Window: 31 * day}
	server, calls, windows := bulkServer(t, 10*day, 0)
	resp, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || len(resp.Data) != 12 {
		t.Fatalf("calls=%d points=%d, want 3 (12d rejected, then 6d + 6d) and 12", calls.Load(), len(resp.Data))
	}
	if (*windows)[1].Until.Sub((*windows)[1].Since) != 6*day {
		t.Errorf("windows=%v, want the second one to be 6 days", *windows)
	}
}

func TestBulkPlanWindows(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(70 * day), Interval: "24h", Window: 31 * day}
	windows := plan.Windows()
	if len(windows) != 3 {
		t.Fatalf("windows = %v", windows)
	}
	for i := 1; i < len(windows); i++ {
		if !windows[i].Since.Equal(windows[i-1].Until) {
			t.Errorf("gap or overlap between %v and %v", windows[i-1], windows[i])
		}
	}
	if !windows[0].Since.Equal(planSince) || !windows[2].Until.Equal(plan.Until) || windows[2].Until.Sub(windows[2].Since) != 8*day {
		t.Errorf("bounds: %v", windows)
	}
	if !strings.Contains(plan.RangeError().Error(), "3 requests") || !strings.Contains(plan.Describe(), "3 requests of up to 31 days (24h)") {
		t.Errorf("messages: %v / %s", plan.RangeError(), plan.Describe())
	}
	if one := (BulkPlan{Since: planSince, Until: planSince.Add(day), Window: 31 * day}).Windows(); len(one) != 1 {
		t.Errorf("one-day range should be one window, got %d", len(one))
	}
}

// bulkServer answers one daily BTC point per day of the requested window and
// records the windows it was asked for. Ranges above maxRange get the API's
// "too large time range" error.
func bulkServer(t *testing.T, maxRange time.Duration, failAt int) (*httptest.Server, *atomic.Int32, *[]Window) {
	t.Helper()
	var calls atomic.Int32
	var windows []Window
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		s, _ := strconv.ParseInt(r.URL.Query().Get("s"), 10, 64)
		u, _ := strconv.ParseInt(r.URL.Query().Get("u"), 10, 64)
		since, until := time.Unix(s, 0).UTC(), time.Unix(u, 0).UTC()
		windows = append(windows, Window{since, until})
		if until.Sub(since) > maxRange {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"message":"too large time range (%s) requested (maximum allowed is %s)"}`, until.Sub(since), maxRange)
			return
		}
		if n == failAt {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"metric not in your plan"}`)
			return
		}
		var points []string
		for day := since.Truncate(24 * time.Hour); day.Add(24*time.Hour).Compare(until) <= 0; day = day.Add(24 * time.Hour) {
			points = append(points, fmt.Sprintf(`{"t":%d,"bulk":[{"a":"BTC","v":1}]}`, day.Unix()))
		}
		fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(points, ","))
	}))
	t.Cleanup(server.Close)
	return server, &calls, &windows
}

func newTestClient(server *httptest.Server) *Client {
	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	return client
}

func TestGetMetricBulkSplit(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(70 * day), Interval: "24h", Window: 31 * day}
	server, calls, _ := bulkServer(t, 31*day, 0)
	resp, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || len(resp.Data) != 70 {
		t.Fatalf("calls=%d points=%d, want 3 and 70", calls.Load(), len(resp.Data))
	}
	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i].T-resp.Data[i-1].T != int64(day.Seconds()) {
			t.Fatalf("gap or duplicate at %d: %d -> %d", i, resp.Data[i-1].T, resp.Data[i].T)
		}
	}
}

func TestGetMetricBulkSplitShrinksWindow(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(31 * day), Interval: "24h", Window: 31 * day}
	// The server allows only 10 days: 31 -> 15.5 -> 7.75 days, two shrinks.
	server, calls, windows := bulkServer(t, 10*day, 0)
	resp, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 31 {
		t.Fatalf("points=%d, want 31", len(resp.Data))
	}
	if (*windows)[0].Until.Sub((*windows)[0].Since) != 31*day || (*windows)[2].Until.Sub((*windows)[2].Since) > 10*day {
		t.Errorf("window sizes: %v", *windows)
	}
	t.Logf("calls=%d", calls.Load())

	// A limit below what three halvings reach is an error.
	tiny, _, _ := bulkServer(t, 2*day, 0)
	if _, err := newTestClient(tiny).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan); err == nil || !strings.Contains(err.Error(), "too large time range") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetMetricBulkSplitAbortsOnFailure(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(70 * day), Interval: "24h", Window: 31 * day}
	server, calls, _ := bulkServer(t, 31*day, 2)
	resp, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err == nil || resp != nil {
		t.Fatalf("expected an error and no data, got %v %v", resp, err)
	}
	for _, want := range []string{"window 2 of about 3", "2024-02-01..2024-03-03", "metric not in your plan", "--since 2024-02-01"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if calls.Load() != 2 {
		t.Errorf("calls=%d, want 2 (no requests after the failure)", calls.Load())
	}
}

func TestWindowParams(t *testing.T) {
	params := map[string]string{"s": "1", "u": "2", "i": "24h"}
	w := Window{planSince, planSince.Add(day)}
	out := WindowParams(params, w)
	if out["s"] != unix(planSince) || out["u"] != unix(planSince.Add(day)) || out["i"] != "24h" || params["s"] != "1" {
		t.Errorf("out=%v params=%v", out, params)
	}
}

// Should windows ever overlap, the later window's point replaces the earlier
// one instead of being dropped.
func TestGetMetricBulkSplitKeepsLaterPointOnOverlap(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(2 * day), Interval: "24h", Window: day}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		boundary := planSince.Add(day).Unix()
		if calls.Add(1) == 1 {
			// The first window wrongly repeats the boundary point with one entry.
			fmt.Fprintf(w, `{"data":[{"t":%d,"bulk":[{"a":"BTC","v":1}]},{"t":%d,"bulk":[{"a":"BTC","v":2}]}]}`, planSince.Unix(), boundary)
			return
		}
		fmt.Fprintf(w, `{"data":[{"t":%d,"bulk":[{"a":"BTC","v":2},{"a":"ETH","v":3}]}]}`, boundary)
	}))
	t.Cleanup(server.Close)
	resp, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 || len(resp.Data[1].Bulk) != 2 {
		t.Fatalf("points=%d boundary entries=%d, want 2 and 2 (from the later window)", len(resp.Data), len(resp.Data[1].Bulk))
	}
}

func TestIsRangeTooLarge(t *testing.T) {
	for _, tt := range []struct {
		status int
		detail string
		want   bool
	}{
		{400, "too large time range (155664h0m0s) requested (maximum allowed is 744h0m0s)", true},
		{400, "Requested Time Range is too large", true},
		{400, "parameter a=NOPE is invalid", false},
		{403, "time range not in your plan", false},
	} {
		if got := isRangeTooLarge(&glassnode.APIError{StatusCode: tt.status, Detail: tt.detail}); got != tt.want {
			t.Errorf("isRangeTooLarge(%d %q) = %v, want %v", tt.status, tt.detail, got, tt.want)
		}
	}
}

func TestGetMetricBulkSplitErrorAfterHalving(t *testing.T) {
	// Hourly data from 06:00: 20 days need two 10-day windows. The server
	// accepts only 5 days, so the window is halved once, then the third
	// request fails.
	since := planSince.Add(6 * time.Hour)
	plan := BulkPlan{Since: since, Until: since.Add(20 * day), Interval: "1h", Window: 10 * day}
	server, _, _ := bulkServer(t, 5*day, 4) // call 1 too large; calls 2-3 succeed; call 4 fails
	_, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"window 3 of about 4", "2024-01-11T06:00:00Z..2024-01-16T06:00:00Z", "--since 2024-01-11T06:00:00Z"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestFormatBound(t *testing.T) {
	if got := formatBound(planSince); got != "2024-01-01" {
		t.Errorf("midnight: %s", got)
	}
	if got := formatBound(planSince.Add(90 * time.Minute)); got != "2024-01-01T01:30:00Z" {
		t.Errorf("intraday: %s", got)
	}
}

// At monthly resolution a 93-day window may be halved once (46 days, still
// over one interval) but not twice (23 days would return no points).
func TestGetMetricBulkSplitDoesNotHalveBelowAnInterval(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(93 * day), Interval: "1month", Window: 93 * day}
	server, calls, windows := bulkServer(t, 40*day, 0) // even 46 days is rejected
	_, err := newTestClient(server).GetMetricBulkSplit(context.Background(), "/market/price", map[string]string{"s": unix(plan.Since)}, nil, plan)
	if err == nil || !strings.Contains(err.Error(), "time range") {
		t.Fatalf("err = %v, want the API's too-large error once halving is exhausted", err)
	}
	if calls.Load() != 2 || (*windows)[1].Until.Sub((*windows)[1].Since) != 46*day+12*time.Hour {
		t.Errorf("calls=%d windows=%v, want 93d then 46.5d and no 23d window", calls.Load(), *windows)
	}
}

func TestRangeMessagesKeepIntradayBounds(t *testing.T) {
	since := planSince.Add(6 * time.Hour)
	plan := BulkPlan{Since: since, Until: since.Add(19 * day), Interval: "1h", Window: 10 * day}
	for _, msg := range []string{plan.RangeError().Error(), plan.Describe()} {
		if !strings.Contains(msg, "2024-01-01T06:00:00Z..2024-01-20T06:00:00Z") {
			t.Errorf("message drops the time of day: %s", msg)
		}
	}
}
