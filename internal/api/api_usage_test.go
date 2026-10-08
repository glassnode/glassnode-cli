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
	if out.CreditsLimit() != 1500000 {
		t.Errorf("CreditsLimit() = %d, want 1500000 (max addon value)", out.CreditsLimit())
	}
	sum := out.Summary(CreditsPeriodMonth)
	if sum.CreditsUsed != 6 || sum.CreditsLimit != 1500000 || sum.CreditsLeft != 1500000-6 {
		t.Errorf("Summary() = %+v, want creditsUsed=6 creditsLimit=1500000 creditsLeft=%d", sum, 1500000-6)
	}
	if sum.CreditsPeriod != CreditsPeriodMonth {
		t.Errorf("Summary().CreditsPeriod = %q, want %q", sum.CreditsPeriod, CreditsPeriodMonth)
	}
}

func TestSummary_PeriodIsCarriedThrough(t *testing.T) {
	usage := &APIUsageResponse{CreditsUsed: 10, APIAddons: []APIAddon{{Value: 100}}}

	for _, period := range []CreditsPeriod{CreditsPeriodDay, CreditsPeriodMonth} {
		sum := usage.Summary(period)
		if sum.CreditsPeriod != period {
			t.Errorf("Summary(%q).CreditsPeriod = %q, want %q", period, sum.CreditsPeriod, period)
		}
	}
}

func TestSummary_CreditsLeftNeverNegative(t *testing.T) {
	usage := &APIUsageResponse{CreditsUsed: 500, APIAddons: []APIAddon{{Value: 100}}}

	if got := usage.Summary(CreditsPeriodDay).CreditsLeft; got != 0 {
		t.Errorf("CreditsLeft = %d, want 0 when usage exceeds the limit", got)
	}
}

func TestSummary_MarshalsCreditsLimitAndPeriod(t *testing.T) {
	usage := &APIUsageResponse{CreditsUsed: 6, APIAddons: []APIAddon{{Value: 50}}}

	body, err := json.Marshal(usage.Summary(CreditsPeriodDay))
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	got := string(body)
	for _, want := range []string{`"creditsLimit":50`, `"creditsPeriod":"day"`, `"creditsLeft":44`, `"creditsUsed":6`} {
		if !strings.Contains(got, want) {
			t.Errorf("summary JSON %s, want it to contain %s", got, want)
		}
	}
	if strings.Contains(got, "creditsPerMonth") {
		t.Errorf("summary JSON %s must not contain the old creditsPerMonth key", got)
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
