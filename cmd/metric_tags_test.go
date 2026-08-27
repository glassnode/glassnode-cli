package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricTags_DryRun_ShowsEndpointWithoutFilter(t *testing.T) {
	stdout, stderr, err := runCLINames(t, "https://api.example.com", "metric", "tags", "--dry-run")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "/v1/metadata/tags") {
		t.Errorf("URL should contain the endpoint path: %s", stdout)
	}
	if strings.Contains(stdout, "filter=") {
		t.Errorf("URL should not contain a filter param: %s", stdout)
	}
}

func TestMetricTags_ListsNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metadata/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"on-chain"},{"name":"spot"}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "metric", "tags", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 2 || got[0] != "on-chain" || got[1] != "spot" {
		t.Errorf("got %v", got)
	}
}

func TestMetricTags_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLINames(t, server.URL, "metric", "tags", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty list", got)
	}
}
