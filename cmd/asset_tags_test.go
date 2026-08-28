package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetTags_DryRun_IncludesEndpointAndFilter(t *testing.T) {
	stdout, stderr, err := runCLINames(t, "https://api.example.com", "asset", "tags", "--filter", `asset.id==BTC`, "--dry-run")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "/v1/metadata/assets/tags") {
		t.Errorf("URL should contain the endpoint path: %s", stdout)
	}
	if !strings.Contains(stdout, "filter=") {
		t.Errorf("URL should contain the filter param: %s", stdout)
	}
}

func TestAssetTags_ListsNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metadata/assets/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"erc20"},{"name":"stablecoin"}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "asset", "tags", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 2 || got[0] != "erc20" || got[1] != "stablecoin" {
		t.Errorf("got %v", got)
	}
}

func TestAssetTags_PassesNormalizedFilter(t *testing.T) {
	var gotFilter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"stablecoin"}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "asset", "tags", "--filter", `asset.id==BTC`, "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if gotFilter != `asset.id=="BTC"` {
		t.Errorf("filter = %q, want normalized CEL string literal", gotFilter)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 1 || got[0] != "stablecoin" {
		t.Errorf("got %v", got)
	}
}
