package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// moduleRoot returns the path to the repository root (parent of cmd/).
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..")
}

// goToolchainCacheDirs returns GOMODCACHE and GOCACHE for the running `go` tool. When tests set
// a synthetic HOME (to isolate ~/.gn), nested `go run` would otherwise default caches under that
// HOME; the module cache uses read-only files and breaks t.TempDir cleanup on CI.
var toolchainCaches struct {
	once         sync.Once
	gomodcache   string
	gocache      string
	resolveError error
}

type goConfig struct {
	GOMODCACHE string `json:"GOMODCACHE"`
	GOCACHE    string `json:"GOCACHE"`
}

func goToolchainCacheDirs() (gomodcache, gocache string, err error) {
	toolchainCaches.once.Do(func() {
		cmd := exec.Command("go", "env", "-json", "GOMODCACHE", "GOCACHE")
		cmd.Env = os.Environ()
		out, e := cmd.Output()
		if e != nil {
			toolchainCaches.resolveError = e
			return
		}
		var gc goConfig
		if e := json.Unmarshal(out, &gc); e != nil {
			toolchainCaches.resolveError = e
			return
		}

		toolchainCaches.gomodcache = gc.GOMODCACHE
		toolchainCaches.gocache = gc.GOCACHE
	})
	return toolchainCaches.gomodcache, toolchainCaches.gocache, toolchainCaches.resolveError
}

// runCLI runs "go run ." with the given args from the module root.
// Returns stdout, stderr, and exit error (nil if exit code 0).
func runCLI(t *testing.T, env []string, args ...string) (string, string, error) {
	t.Helper()
	root := moduleRoot(t)
	gomodcache, gocache, err := goToolchainCacheDirs()
	if err != nil {
		t.Fatalf("go env GOMODCACHE/GOCACHE: %v", err)
	}
	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "GOMODCACHE="+gomodcache, "GOCACHE="+gocache)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.String(), errBuf.String(), runErr
}

func TestMetricGet_DryRun_PrintsURLWithoutKey(t *testing.T) {
	stdout, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
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
	if strings.Contains(stdout, "api_key") {
		t.Errorf("stdout must not contain an api_key parameter: %s", stdout)
	}
	if !strings.Contains(stderr, "X-Api-Key") {
		t.Errorf("stderr should say the key is sent in the X-Api-Key header: %s", stderr)
	}
	if !strings.Contains(stdout, "/v1/metrics") || !strings.Contains(stdout, "market/price_usd_close") {
		t.Errorf("stdout should contain metric path: %s", stdout)
	}
}

func TestMetricGet_DryRun_URLContainsParams(t *testing.T) {
	stdout, _, err := runCLI(t, []string{"HOME=" + t.TempDir()},
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
	// No --api-key flag, clean HOME (so no config file), and GLASSNODE_API_KEY explicitly
	// unset: the CLI must error out at credential resolution without making any HTTP call.
	stdout, stderr, err := runCLI(t,
		[]string{"HOME=" + t.TempDir(), "GLASSNODE_API_KEY="},
		"metric", "get", "/market/price_usd_close",
	)
	if err == nil {
		t.Fatalf("expected non-zero exit; stdout: %s", stdout)
	}
	if !strings.Contains(stderr, "no credentials") && !strings.Contains(stderr, "API key") && !strings.Contains(stderr, "gn login") {
		t.Errorf("expected credential error in stderr, got: %q", stderr)
	}
}

func TestMetricGet_InvalidSince_Fails(t *testing.T) {
	_, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
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
	_, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
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

// Without --split the CLI sends the one planned request and reports the API's
// rejection; it does not halve the window and multiply billed requests.
func TestMetricGet_Bulk_WithoutSplitSendsOneRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"too large time range (744h0m0s) requested (maximum allowed is 240h0m0s)"}`)
	}))
	t.Cleanup(server.Close)
	_, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir(), "GLASSNODE_BASE_URL=" + server.URL},
		"metric", "get", "/market/price_usd_close/bulk",
		"--api-key", "k", "--asset", "BTC", "--since", "31d",
	)
	if err == nil {
		t.Fatal("expected non-zero exit when the API rejects the range")
	}
	if calls.Load() != 1 {
		t.Errorf("requests = %d, want exactly 1 without --split", calls.Load())
	}
	for _, want := range []string{"too large time range", "add --split"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q lacks %q", stderr, want)
		}
	}
}

// The one request sent without --split carries the planned bounds, the same
// ones --dry-run prints: a future --until is capped at now, so the API never
// sees a range the plan did not validate.
func TestMetricGet_Bulk_WithoutSplitSendsPlannedBounds(t *testing.T) {
	start := time.Now()
	var calls atomic.Int32
	var until string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		until = r.URL.Query().Get("u")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	_, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir(), "GLASSNODE_BASE_URL=" + server.URL},
		"metric", "get", "/market/price_usd_close/bulk",
		"--api-key", "k", "--asset", "BTC", "--since", "20d", "--until", "2999-01-01",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	u, convErr := strconv.ParseInt(until, 10, 64)
	if calls.Load() != 1 || convErr != nil || u < start.Unix() || u > time.Now().Unix() {
		t.Errorf("calls=%d u=%q, want one request with u capped at now", calls.Load(), until)
	}
}

func TestMetricGet_SplitOnBaseMetricIsIgnoredWithANote(t *testing.T) {
	stdout, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
		"metric", "get", "/market/price_usd_close", "--api-key", "k", "--since", "400d", "--split", "--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stderr, "--split applies to bulk paths only") || strings.Contains(stdout, "split") {
		t.Errorf("stderr=%q stdout=%q, want the note on stderr and a plain URL on stdout", stderr, stdout)
	}
}

// A bulk dry run prints one URL per window, the header note once, and no
// split summary, since nothing is fetched.
func TestMetricGet_BulkDryRun_NoteOnce(t *testing.T) {
	stdout, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
		"metric", "get", "/market/price_usd_close/bulk", "--api-key", "k", "--asset", "BTC",
		"--since", "2024-01-01", "--until", "2024-03-01", "--split", "--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}
	if strings.Count(stdout, "/v1/metrics/market/price_usd_close/bulk?") != 2 {
		t.Errorf("stdout should hold two window URLs:\n%s", stdout)
	}
	if strings.Count(stderr, "# credentials") != 1 || strings.Contains(stderr, "splitting") {
		t.Errorf("stderr should carry the note once and no split summary:\n%s", stderr)
	}
}
