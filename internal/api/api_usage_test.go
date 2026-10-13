package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

func TestGetAPIUsage(t *testing.T) {
	fixture, err := os.ReadFile("testdata/api_usage.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var gotPath, gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-Api-Key")
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
		t.Errorf("X-Api-Key = %q, want my-key", gotAPIKey)
	}
	if out.CreditsUsed != 6 || out.DailyRequestsUsed != 2 {
		t.Errorf("CreditsUsed = %d, DailyRequestsUsed = %d, want 6 and 2", out.CreditsUsed, out.DailyRequestsUsed)
	}
	if len(out.APIAddons) != 1 {
		t.Fatalf("len(APIAddons) = %d, want 1", len(out.APIAddons))
	}
	if addon := out.APIAddons[0]; addon.Period != CreditsPeriodMonthly || addon.RPM != 100 {
		t.Errorf("APIAddons[0] = %+v, want period=monthly rpm=100", addon)
	}
	sum := out.Summary()
	if *sum.CreditsUsed != 6 || sum.CreditsLimit != 1500000 || *sum.CreditsLeft != 1500000-6 || sum.CreditsPeriod != CreditsPeriodMonthly {
		t.Errorf("Summary() = %+v, want creditsUsed=6 creditsLimit=1500000 creditsLeft=%d monthly", sum, 1500000-6)
	}
}

func usage(creditsUsed, dailyRequestsUsed int, addons ...APIAddon) *APIUsageResponse {
	return &APIUsageResponse{glassnode.APIUsage{CreditsUsed: creditsUsed, DailyRequestsUsed: dailyRequestsUsed, APIAddons: addons}}
}

func TestCreditsPeriod_ComesFromTheAddon(t *testing.T) {
	tests := []struct {
		name   string
		addons []APIAddon
		want   CreditsPeriod
	}{
		{"daily", []APIAddon{{Value: 50, Period: "daily"}}, CreditsPeriodDaily},
		{"monthly", []APIAddon{{Value: 1500000, Period: "monthly"}}, CreditsPeriodMonthly},
		{"an absent period is monthly", []APIAddon{{Value: 50}}, CreditsPeriodMonthly},
		{"no addons is monthly", nil, CreditsPeriodMonthly},
		{"a period outside the enum is reported as sent", []APIAddon{{Value: 50, Period: "weekly"}}, "weekly"},
		{"mixed periods are reported as mixed", []APIAddon{{Value: 50, Period: "daily"}, {Value: 5000000, Period: "monthly"}}, "mixed"},
		{"an absent period counts as monthly when mixed", []APIAddon{{Value: 50}, {Value: 5000000, Period: "monthly"}, {Value: 7, Period: "weekly"}}, "mixed"},
		{"an absent period never surfaces as empty", []APIAddon{{Value: 50}, {Value: 60, Period: "weekly"}}, "mixed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usage(0, 0, tt.addons...).Summary().CreditsPeriod; got != tt.want {
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
		addon    APIAddon
		wantUsed int
		wantLeft int
	}{
		{"daily counts the day's requests", APIAddon{Value: 50, Period: "daily"}, 2, 50 - 2},
		{"monthly counts the running total", APIAddon{Value: 50000, Period: "monthly"}, 4000, 50000 - 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := usage(4000, 2, tt.addon).Summary()
			if *sum.CreditsUsed != tt.wantUsed || *sum.CreditsLeft != tt.wantLeft {
				t.Errorf("Summary() used=%d left=%d, want %d and %d", *sum.CreditsUsed, *sum.CreditsLeft, tt.wantUsed, tt.wantLeft)
			}
		})
	}
}

func TestSummary_NoAddons(t *testing.T) {
	sum := usage(3, 0).Summary()
	if sum.CreditsLimit != 0 || *sum.CreditsLeft != 0 || *sum.CreditsUsed != 3 {
		t.Errorf("Summary() = %+v, want no limit, nothing left, 3 used", sum)
	}
}

func TestSummary_CreditsLeftNeverNegative(t *testing.T) {
	if got := *usage(0, 500, APIAddon{Value: 100, Period: "daily"}).Summary().CreditsLeft; got != 0 {
		t.Errorf("CreditsLeft = %d, want 0 when usage exceeds the limit", got)
	}
}

// An unknown period must not be measured against a counter of another period:
// the limit and the period are reported, the usage fields are left out.
func TestSummary_UnknownPeriodOmitsUsage(t *testing.T) {
	body, err := json.Marshal(usage(9000, 6, APIAddon{Value: 300, Period: "weekly"}).Summary())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), `{"creditsLimit":300,"creditsPeriod":"weekly"}`; got != want {
		t.Errorf("summary JSON %s, want %s", got, want)
	}
}

func TestSummary_MarshalsCreditsLimitAndPeriod(t *testing.T) {
	body, err := json.Marshal(usage(9000, 6, APIAddon{Value: 50, Period: "daily"}).Summary())
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

	var decode *glassnode.DecodeError
	if !errors.As(err, &decode) || decode.Endpoint != "/v1/user/api_usage" {
		t.Errorf("error = %v, want typed API usage decode error", err)
	}
}
