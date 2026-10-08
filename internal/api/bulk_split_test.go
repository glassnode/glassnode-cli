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
		if err == nil || !strings.Contains(err.Error(), "--since") || !strings.Contains(err.Error(), "31 days") {
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
	t.Run("until in the future is capped at now", func(t *testing.T) {
		plan, err := PlanBulk(map[string]string{"s": unix(planSince), "u": unix(planNow.Add(400 * day))}, planNow)
		if err != nil || !plan.Until.Equal(planNow) {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
	})
	t.Run("since must precede until", func(t *testing.T) {
		if _, err := PlanBulk(map[string]string{"s": unix(planNow), "u": unix(planSince)}, planNow); err == nil {
			t.Fatal("accepted since >= until")
		}
		if _, err := PlanBulk(map[string]string{"s": unix(planNow.Add(day))}, planNow); err == nil {
			t.Fatal("accepted since in the future")
		}
	})
}

func TestBulkPlanWindows(t *testing.T) {
	plan := BulkPlan{Since: planSince, Until: planSince.Add(70 * day), Interval: "24h", Window: 31 * day}
	windows := plan.Windows()
	if len(windows) != 3 || plan.Fits() {
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
	if one := (BulkPlan{Since: planSince, Until: planSince.Add(day), Window: 31 * day}); !one.Fits() {
		t.Error("one-day range should fit")
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
	for _, want := range []string{"window 2/3", "2024-02-01..2024-03-03", "metric not in your plan", "--since 2024-02-01"} {
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
