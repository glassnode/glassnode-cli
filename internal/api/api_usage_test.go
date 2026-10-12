package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGetAPIUsage(t *testing.T) {
	fixture, err := os.ReadFile("testdata/api_usage.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var gotPath, gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.URL.Query().Get("api_key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient("my-key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	out, err := client.GetAPIUsage(context.Background())
	if err != nil {
		t.Fatalf("GetAPIUsage: %v", err)
	}

	if gotPath != "/v1/user/api_usage" {
		t.Errorf("path = %q, want /v1/user/api_usage", gotPath)
	}
	if gotAPIKey != "my-key" {
		t.Errorf("api_key = %q, want my-key", gotAPIKey)
	}
	if out.CreditsUsed != 6 {
		t.Errorf("CreditsUsed = %d, want 6", out.CreditsUsed)
	}
	if len(out.APIAddons) != 1 {
		t.Errorf("len(APIAddons) = %d, want 1", len(out.APIAddons))
	}
	if out.CustomerID != 4568 {
		t.Errorf("CustomerID = %d, want 4568", out.CustomerID)
	}
	if out.DailyRequestsUsed != 2 {
		t.Errorf("DailyRequestsUsed = %d, want 2", out.DailyRequestsUsed)
	}
	if addon := out.APIAddons[0]; addon.Period != "monthly" || addon.RPM != 100 || addon.Version != "v3" {
		t.Errorf("APIAddons[0] = %+v, want period=monthly rpm=100 version=v3", addon)
	}
	if out.CreditsLimit() != 1500000 {
		t.Errorf("CreditsLimit() = %d, want 1500000 (max addon value)", out.CreditsLimit())
	}
	sum := out.Summary()
	if sum.CreditsUsed != 6 || sum.CreditsLimit != 1500000 || sum.CreditsLeft != 1500000-6 {
		t.Errorf("Summary() = %+v, want creditsUsed=6 creditsLimit=1500000 creditsLeft=%d", sum, 1500000-6)
	}
	if sum.CreditsPeriod != CreditsPeriodMonthly {
		t.Errorf("Summary().CreditsPeriod = %q, want %q", sum.CreditsPeriod, CreditsPeriodMonthly)
	}
}

func TestCreditsPeriod_ComesFromTheAddon(t *testing.T) {
	tests := []struct {
		name   string
		addons []APIAddon
		want   CreditsPeriod
	}{
		{"daily", []APIAddon{{Value: 50, Period: "daily"}}, CreditsPeriodDaily},
		{"monthly", []APIAddon{{Value: 1500000, Period: "monthly"}}, CreditsPeriodMonthly},
		{"an absent period falls back to monthly", []APIAddon{{Value: 50}}, CreditsPeriodMonthly},
		{"a period outside the enum falls back to monthly", []APIAddon{{Value: 50, Period: "weekly"}}, CreditsPeriodMonthly},
		{"no addons falls back to monthly", nil, CreditsPeriodMonthly},
		{"a mixed-case period still maps", []APIAddon{{Value: 50, Period: "Daily"}}, CreditsPeriodDaily},
		{"a padded period still maps", []APIAddon{{Value: 50, Period: " monthly "}}, CreditsPeriodMonthly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &APIUsageResponse{APIAddons: tt.addons}
			if got := usage.CreditsPeriod(); got != tt.want {
				t.Errorf("CreditsPeriod() = %q, want %q", got, tt.want)
			}
			if got := usage.Summary().CreditsPeriod; got != tt.want {
				t.Errorf("Summary().CreditsPeriod = %q, want %q", got, tt.want)
			}
		})
	}
}

// A daily allowance has to be measured against the day's usage, not the running
// total, or an advanced account reports no credits left within days of use.
func TestSummary_UsageIsScopedToThePeriod(t *testing.T) {
	tests := []struct {
		name     string
		period   string
		wantUsed int
		wantLeft int
	}{
		{"daily counts the day's requests", "daily", 2, 50 - 2},
		{"monthly counts the running total", "monthly", 4000, 50000 - 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := 50
			if tt.period == string(CreditsPeriodMonthly) {
				limit = 50000
			}
			usage := &APIUsageResponse{
				CreditsUsed:       4000,
				DailyRequestsUsed: 2,
				APIAddons:         []APIAddon{{Value: limit, Period: tt.period}},
			}

			if got := usage.CreditsUsedInPeriod(); got != tt.wantUsed {
				t.Errorf("CreditsUsedInPeriod() = %d, want %d", got, tt.wantUsed)
			}
			sum := usage.Summary()
			if sum.CreditsUsed != tt.wantUsed {
				t.Errorf("CreditsUsed = %d, want %d", sum.CreditsUsed, tt.wantUsed)
			}
			if sum.CreditsLeft != tt.wantLeft {
				t.Errorf("CreditsLeft = %d, want %d", sum.CreditsLeft, tt.wantLeft)
			}
		})
	}
}

func TestCreditsLimit_NoAddons(t *testing.T) {
	usage := &APIUsageResponse{CreditsUsed: 3}

	if got := usage.CreditsLimit(); got != 0 {
		t.Errorf("CreditsLimit() = %d, want 0 without addons", got)
	}
	if got := usage.Summary().CreditsLeft; got != 0 {
		t.Errorf("CreditsLeft = %d, want 0 without addons", got)
	}
}

func TestSummary_CreditsLeftNeverNegative(t *testing.T) {
	usage := &APIUsageResponse{
		DailyRequestsUsed: 500,
		APIAddons:         []APIAddon{{Value: 100, Period: "daily"}},
	}

	if got := usage.Summary().CreditsLeft; got != 0 {
		t.Errorf("CreditsLeft = %d, want 0 when usage exceeds the limit", got)
	}
}

func TestSummary_MarshalsCreditsLimitAndPeriod(t *testing.T) {
	usage := &APIUsageResponse{
		CreditsUsed:       9000,
		DailyRequestsUsed: 6,
		APIAddons:         []APIAddon{{Value: 50, Period: "daily"}},
	}

	body, err := json.Marshal(usage.Summary())
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	got := string(body)
	for _, want := range []string{`"creditsLimit":50`, `"creditsPeriod":"daily"`, `"creditsLeft":44`, `"creditsUsed":6`} {
		if !strings.Contains(got, want) {
			t.Errorf("summary JSON %s, want it to contain %s", got, want)
		}
	}
	if strings.Contains(got, "creditsPerMonth") {
		t.Errorf("summary JSON %s must not contain the old creditsPerMonth key", got)
	}
	// The running total must not leak into a daily summary.
	if strings.Contains(got, "9000") {
		t.Errorf("summary JSON %s must not report the running total on a daily allowance", got)
	}
}

func TestGetAPIUsage_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.GetAPIUsage(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decoding API usage response") {
		t.Errorf("error = %v, want wrapping decode message", err)
	}
}
