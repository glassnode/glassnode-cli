package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runCLIAssetList(t *testing.T, baseURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "..")
	cmdArgs := append([]string{"run", "."}, append([]string{"asset", "list", "--api-key", "test-key"}, args...)...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = root
	env := append(os.Environ(), "GLASSNODE_BASE_URL="+baseURL)
	cmd.Env = env
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.String(), errBuf.String(), runErr
}

func TestNormalizeFilter(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`asset.id==BTC`, `asset.id=="BTC"`},
		{`asset.id==ETH`, `asset.id=="ETH"`},
		{`asset.symbol==BTC`, `asset.symbol=="BTC"`},
		{``, ``},
		{`asset.semantic_tags.exists(tag,tag=='stablecoin')`, `asset.semantic_tags.exists(tag,tag=='stablecoin')`},
		{`asset.id=="BTC"`, `asset.id=="BTC"`},
	}
	for _, tt := range tests {
		got := normalizeFilter(tt.in)
		if got != tt.want {
			t.Errorf("normalizeFilter(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAssetList_DryRun_FilterHasQuotedRHS(t *testing.T) {
	stdout, stderr, err := runCLIAssetList(t, "https://api.example.com", "--filter", `asset.id==BTC`, "--dry-run")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	// Filter should be sent as asset.id=="BTC" (URL-encoded: %22 = ")
	if !strings.Contains(stdout, "filter=") {
		t.Errorf("URL should contain filter param: %s", stdout)
	}
	if !strings.Contains(stdout, "%22BTC%22") && !strings.Contains(stdout, `"BTC"`) {
		t.Errorf("filter value should have quoted BTC for CEL string literal: %s", stdout)
	}
}

func TestAssetList_Prune_OutputsArrayOfObjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"BTC","symbol":"BTC","name":"Bitcoin","asset_type":"BLOCKCHAIN","categories":["spot"]},{"id":"ETH","symbol":"ETH","name":"Ethereum","asset_type":"BLOCKCHAIN","categories":["spot"]}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLIAssetList(t, server.URL, "--prune", "id", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 2 {
		t.Fatalf("got %d objects, want 2", len(got))
	}
	if got[0]["id"] != "BTC" || got[1]["id"] != "ETH" {
		t.Errorf("got %v", got)
	}
	if _, ok := got[0]["name"]; ok {
		t.Error("pruned output should not contain name")
	}
}

func TestAssetList_PruneMultipleFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"BTC","symbol":"BTC","name":"Bitcoin","asset_type":"BLOCKCHAIN","categories":["spot"]}]}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLIAssetList(t, server.URL, "--prune", "id,symbol,name", "-o", "json")
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(got) != 1 {
		t.Fatalf("got %d objects, want 1", len(got))
	}
	if got[0]["id"] != "BTC" || got[0]["symbol"] != "BTC" || got[0]["name"] != "Bitcoin" {
		t.Errorf("got %v", got[0])
	}
	if _, ok := got[0]["asset_type"]; ok {
		t.Error("pruned output should not contain asset_type")
	}
}
