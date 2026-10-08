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

func TestGetUserInfo(t *testing.T) {
	fixture, err := os.ReadFile("testdata/user_info.json")
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

	out, err := client.GetUserInfo(context.Background())
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}

	if gotPath != "/v1/user/info" {
		t.Errorf("path = %q, want /v1/user/info", gotPath)
	}
	if gotAPIKey != "my-key" {
		t.Errorf("api_key = %q, want my-key", gotAPIKey)
	}
	if out.UUID != "cus_000000000000TEST" {
		t.Errorf("UUID = %q, want cus_000000000000TEST", out.UUID)
	}
	if len(out.Products) != 1 || out.Products[0] != "advanced" {
		t.Errorf("Products = %v, want [advanced]", out.Products)
	}
	if got := out.CreditsPeriod(); got != CreditsPeriodDay {
		t.Errorf("CreditsPeriod() = %q, want %q for the advanced product", got, CreditsPeriodDay)
	}
}

func TestUserInfoCreditsPeriod(t *testing.T) {
	tests := []struct {
		name     string
		products []string
		want     CreditsPeriod
	}{
		{"advanced is daily", []string{"advanced"}, CreditsPeriodDay},
		{"advanced alongside others is daily", []string{"professional", "advanced"}, CreditsPeriodDay},
		{"advanced is matched case-insensitively", []string{"Advanced"}, CreditsPeriodDay},
		{"advanced is matched when padded", []string{" advanced "}, CreditsPeriodDay},
		{"professional is monthly", []string{"professional"}, CreditsPeriodMonth},
		{"standard is monthly", []string{"standard"}, CreditsPeriodMonth},
		{"no products is monthly", nil, CreditsPeriodMonth},
		{"a product merely containing advanced is monthly", []string{"advanced-plus"}, CreditsPeriodMonth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &UserInfoResponse{Products: tt.products}
			if got := info.CreditsPeriod(); got != tt.want {
				t.Errorf("CreditsPeriod() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The /v1/user/info payload carries apiKey and searchAPIKey. Nothing in the
// decoded struct may hold a credential, since the summary is printed to stdout.
func TestGetUserInfo_DoesNotDecodeCredentials(t *testing.T) {
	fixture, err := os.ReadFile("testdata/user_info.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient("my-key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	out, err := client.GetUserInfo(context.Background())
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}

	reencoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal user info: %v", err)
	}
	// The canary values the fixture carries in apiKey and searchAPIKey.
	for _, secret := range []string{"do-not-decode-api-key", "do-not-decode-search-api-key", "apiKey", "searchAPIKey"} {
		if strings.Contains(string(reencoded), secret) {
			t.Errorf("decoded user info %s must not contain %q", reencoded, secret)
		}
	}
}

func TestGetUserInfo_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.GetUserInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decoding user info response") {
		t.Errorf("error = %v, want wrapping decode message", err)
	}
}
