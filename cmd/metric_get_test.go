package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// moduleRoot returns the path to the repository root (parent of cmd/).
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..")
}

// runCLI runs "go run ." with the given args from the module root.
// Returns stdout, stderr, and exit error (nil if exit code 0).
func runCLI(t *testing.T, env []string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := moduleRoot(t)
	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.String(), errBuf.String(), runErr
}

func TestMetricGet_DryRun_PrintsURLWithRedactedKey(t *testing.T) {
	stdout, stderr, err := runCLI(t, nil,
		"metric", "get", "/market/price_usd_close",
		"--api-key", "secret-key",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(stdout, "secret-key") {
		t.Errorf("stdout must not contain API key: %s", stdout)
	}
	if !strings.Contains(stdout, "api_key=***") && !strings.Contains(stdout, "api_key=%2A%2A%2A") {
		t.Errorf("stdout should contain redacted api_key: %s", stdout)
	}
	if !strings.Contains(stdout, "/v1/metrics") || !strings.Contains(stdout, "market/price_usd_close") {
		t.Errorf("stdout should contain metric path: %s", stdout)
	}
}

func TestMetricGet_DryRun_URLContainsParams(t *testing.T) {
	stdout, _, err := runCLI(t, nil,
		"metric", "get", "/market/price_usd_close",
		"--api-key", "k",
		"--asset", "BTC",
		"--since", "2024-01-01",
		"--until", "2024-02-01",
		"--interval", "24h",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v", err)
	}
	if !strings.Contains(stdout, "a=BTC") {
		t.Errorf("URL should contain asset: %s", stdout)
	}
	if !strings.Contains(stdout, "s=1704067200") {
		t.Errorf("URL should contain since (2024-01-01): %s", stdout)
	}
	if !strings.Contains(stdout, "u=1706745600") {
		t.Errorf("URL should contain until (2024-02-01): %s", stdout)
	}
	if !strings.Contains(stdout, "i=24h") {
		t.Errorf("URL should contain interval: %s", stdout)
	}
}

func TestMetricGet_MissingAPIKey_Fails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
	}))
	defer server.Close()

	stdout, stderr, err := runCLI(t, []string{"GLASSNODE_BASE_URL=" + server.URL},
		"metric", "get", "/market/price_usd_close",
	)
	if err == nil {
		t.Fatalf("expected non-zero exit; stdout: %s", stdout)
	}
	// Error message may be in stderr (e.g. "HTTP 401: ...") or stdout
	combined := stderr + stdout
	if combined == "" {
		t.Errorf("expected some error output; stderr: %q stdout: %q", stderr, stdout)
	}
}

func TestMetricGet_InvalidSince_Fails(t *testing.T) {
	_, stderr, err := runCLI(t, nil,
		"metric", "get", "/market/price_usd_close",
		"--api-key", "k",
		"--since", "not-a-date",
	)
	if err == nil {
		t.Fatal("expected non-zero exit for invalid --since")
	}
	if !strings.Contains(stderr, "since") && !strings.Contains(stderr, "parsing") && !strings.Contains(stderr, "unrecognized") {
		t.Errorf("stderr should mention since/parsing: %q", stderr)
	}
}

func TestMetricGet_InvalidUntil_Fails(t *testing.T) {
	_, stderr, err := runCLI(t, nil,
		"metric", "get", "/market/price_usd_close",
		"--api-key", "k",
		"--until", "invalid",
	)
	if err == nil {
		t.Fatal("expected non-zero exit for invalid --until")
	}
	if !strings.Contains(stderr, "until") && !strings.Contains(stderr, "parsing") && !strings.Contains(stderr, "unrecognized") {
		t.Errorf("stderr should mention until/parsing: %q", stderr)
	}
}
