package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetBlockchains_DryRun_IncludesEndpointAndFilter(t *testing.T) {
	stdout, stderr, err := runCLINames(t, "https://api.example.com", "asset", "blockchains", "--filter", `asset.id==USDT`, "--dry-run")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "/v1/metadata/assets/blockchains") {
		t.Errorf("URL should contain the endpoint path: %s", stdout)
	}
	if !strings.Contains(stdout, "filter=") {
		t.Errorf("URL should contain the filter param: %s", stdout)
	}
}

func TestAssetBlockchains_ListsNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metadata/assets/blockchains" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"BTC"},{"name":"ETH"}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "asset", "blockchains", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 2 || got[0] != "BTC" || got[1] != "ETH" {
		t.Errorf("got %v", got)
	}
}

func TestAssetBlockchains_PassesNormalizedFilter(t *testing.T) {
	var gotFilter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"ETH"}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "asset", "blockchains", "--filter", `asset.id==USDT`, "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if gotFilter != `asset.id=="USDT"` {
		t.Errorf("filter = %q, want normalized CEL string literal", gotFilter)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 1 || got[0] != "ETH" {
		t.Errorf("got %v", got)
	}
}
