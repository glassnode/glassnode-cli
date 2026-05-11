package api

import (
	"context"
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
	if out.CreditsPerMonth() != 1500000 {
		t.Errorf("CreditsPerMonth() = %d, want 1500000 (max addon value)", out.CreditsPerMonth())
	}
	sum := out.Summary()
	if sum.CreditsUsed != 6 || sum.CreditsPerMonth != 1500000 || sum.CreditsLeft != 1500000-6 {
		t.Errorf("Summary() = %+v, want creditsUsed=6 creditsPerMonth=1500000 creditsLeft=%d", sum, 1500000-6)
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
