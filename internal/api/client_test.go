package api

import (
	"context"
	_ "embed"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/glassnode/glassnode-cli/internal/oauth"
	"github.com/glassnode/glassnode-cli/internal/testhelper"
)

//go:embed testdata/assets.json
var testdataAssets []byte

//go:embed testdata/metrics_list.json
var testdataMetricsList []byte

//go:embed testdata/metric_describe.json
var testdataMetricDescribe []byte

//go:embed testdata/metric_points.json
var testdataMetricPoints []byte

//go:embed testdata/metric_bulk.json
var testdataMetricBulk []byte

func withTempHome(t *testing.T, fn func()) {
	testhelper.WithTempHome(t, fn)
}

func TestResolveAPIKey_FlagValue(t *testing.T) {
	got := ResolveAPIKey("flag-key")
	if got != "flag-key" {
		t.Errorf("got %q, want flag-key", got)
	}
}

func TestResolveAPIKey_EnvVar(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Setenv("GLASSNODE_API_KEY", "env-key")
		t.Cleanup(func() { _ = os.Unsetenv("GLASSNODE_API_KEY") })
		got := ResolveAPIKey("")
		if got != "env-key" {
			t.Errorf("got %q, want env-key", got)
		}
	})
}

func TestResolveAPIKey_Empty(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Unsetenv("GLASSNODE_API_KEY")
		got := ResolveAPIKey("")
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestResolveAPIKey_FromConfigFile(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Unsetenv("GLASSNODE_API_KEY")
		if err := config.Set("api-key", "config-key"); err != nil {
			t.Fatalf("config.Set: %v", err)
		}
		got := ResolveAPIKey("")
		if got != "config-key" {
			t.Errorf("got %q, want config-key", got)
		}
	})
}

func TestResolveAuth_OAuthPreferred(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Unsetenv("GLASSNODE_API_KEY")
		if err := config.Set("api-key", "config-key"); err != nil {
			t.Fatalf("config.Set: %v", err)
		}
		exp := time.Now().UTC().Add(time.Hour)
		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "oauth-access",
			RefreshToken: "oauth-refresh",
			ExpiresAt:    exp,
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		key, bearer, err := ResolveAuth(context.Background(), "flag-key")
		if err != nil {
			t.Fatalf("ResolveAuth: %v", err)
		}
		if key != "" || bearer != "oauth-access" {
			t.Errorf("ResolveAuth: apiKey=%q bearer=%q (want empty key, oauth-access)", key, bearer)
		}
	})
}

func TestResolveAuth_FallbackWhenOAuthExpired(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Unsetenv("GLASSNODE_API_KEY")
		if err := config.Set("api-key", "fallback-key"); err != nil {
			t.Fatalf("config.Set: %v", err)
		}
		past := time.Now().UTC().Add(-time.Hour)
		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "dead",
			RefreshToken: "",
			ExpiresAt:    past,
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		key, bearer, err := ResolveAuth(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveAuth: %v", err)
		}
		if bearer != "" || key != "fallback-key" {
			t.Errorf("ResolveAuth: apiKey=%q bearer=%q", key, bearer)
		}
	})
}

func TestResolveAuth_RefreshesExpiredAccessToken(t *testing.T) {
	withTempHome(t, func() {
		_ = os.Unsetenv("GLASSNODE_API_KEY")
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				t.Errorf("unexpected path %s", r.URL.Path)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Errorf("grant_type=%q", r.Form.Get("grant_type"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200,"token_type":"Bearer"}`))
		}))
		defer ts.Close()

		restore := oauth.OverrideDefaultsForTesting(ts.URL, "https://api.example", "test-client")
		defer restore()

		past := time.Now().UTC().Add(-time.Hour)
		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "dead",
			RefreshToken: "old-refresh",
			ExpiresAt:    past,
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}

		key, bearer, err := ResolveAuth(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveAuth: %v", err)
		}
		if key != "" || bearer != "new-access" {
			t.Errorf("ResolveAuth: apiKey=%q bearer=%q", key, bearer)
		}
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthRefreshToken != "new-refresh" {
			t.Errorf("refresh token not rotated in config: %q", cfg.OAuthRefreshToken)
		}
	})
}

func TestDo_SendsCorrectURL(t *testing.T) {
	var capturedURL, capturedKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		capturedKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient("my-api-key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.Do(context.Background(), "GET", "/v1/test", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if capturedKey != "my-api-key" || strings.Contains(capturedURL, "api_key") {
		t.Errorf("URL %q header %q: the key must travel in X-Api-Key only", capturedURL, capturedKey)
	}
	if !strings.Contains(capturedURL, "a=b") {
		t.Errorf("URL %q missing a=b param", capturedURL)
	}
}

func TestDo_SendsBearerAuthorization(t *testing.T) {
	var authHdr, capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHdr = r.Header.Get("Authorization")
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient("", "secret-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.Do(context.Background(), "GET", "/v1/test", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if authHdr != "Bearer secret-token" {
		t.Errorf("Authorization header %q, want Bearer secret-token", authHdr)
	}
	if strings.Contains(capturedURL, "api_key") {
		t.Errorf("URL should not contain api_key when using bearer: %q", capturedURL)
	}
}

func TestDo_401TriggersRefreshAndRetry(t *testing.T) {
	withTempHome(t, func() {
		var mu sync.Mutex
		var authHdrs []string
		var attempts int
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			authHdrs = append(authHdrs, r.Header.Get("Authorization"))
			attempts++
			attempt := attempts
			mu.Unlock()
			if attempt == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer api.Close()

		idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"rt2","expires_in":3600,"token_type":"Bearer"}`))
		}))
		defer idp.Close()
		restore := oauth.OverrideDefaultsForTesting(idp.URL, "https://api.example", "cid")
		defer restore()

		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "stale",
			RefreshToken: "rt1",
			ExpiresAt:    time.Now().UTC().Add(time.Hour), // appears valid by expiry
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}

		client := NewClient("", "stale")
		client.baseURL = api.URL
		client.httpClient = api.Client()

		body, err := client.Do(context.Background(), "GET", "/v1/anything", nil)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if !strings.Contains(string(body), "ok") {
			t.Errorf("body %q, want contents including 'ok'", string(body))
		}
		if len(authHdrs) != 2 {
			t.Fatalf("expected 2 API attempts, got %d", len(authHdrs))
		}
		if authHdrs[0] != "Bearer stale" || authHdrs[1] != "Bearer fresh" {
			t.Errorf("auth headers %v", authHdrs)
		}
		if _, err := client.Do(context.Background(), "GET", "/v1/anything", nil); err != nil {
			t.Fatal(err)
		}
		if len(authHdrs) != 3 || authHdrs[2] != "Bearer fresh" {
			t.Errorf("refreshed token was not reused: %v", authHdrs)
		}
	})
}

func TestOAuthErrorResponseRedactsRotatedToken(t *testing.T) {
	response := &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("rotated-secret"))}
	response = redactOAuthErrorResponse(response, "rotated-secret")
	body, err := io.ReadAll(response.Body)
	if err != nil || strings.Contains(string(body), "rotated-secret") {
		t.Fatalf("unredacted response: %s %v", body, err)
	}
}

func TestDo_401WithAPIKey_NoRetry(t *testing.T) {
	// When using an API key (not a bearer), a 401 must NOT trigger a refresh attempt; we just
	// surface the error. This guards against silently attempting OAuth refreshes for users who
	// never ran gn login.
	withTempHome(t, func() {
		var mu sync.Mutex
		var attempts int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			attempts++
			mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"bad key"}`))
		}))
		defer server.Close()

		client := NewClient("apikey", "")
		client.baseURL = server.URL
		client.httpClient = server.Client()

		_, err := client.Do(context.Background(), "GET", "/v1/x", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		mu.Lock()
		got := attempts
		mu.Unlock()
		if got != 1 {
			t.Errorf("attempts=%d, want 1 (no retry for API-key auth)", got)
		}
	})
}

func TestDo_Non2xxReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.Do(context.Background(), "GET", "/v1/test", nil)
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestDoWithRepeatedParams(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	repeated := map[string][]string{"a": {"x", "y"}}
	_, err := client.DoWithRepeatedParams(context.Background(), "GET", "/v1/test", nil, repeated)
	if err != nil {
		t.Fatalf("DoWithRepeatedParams: %v", err)
	}
	if !strings.Contains(capturedURL, "a=x") || !strings.Contains(capturedURL, "a=y") {
		t.Errorf("URL %q should contain a=x and a=y", capturedURL)
	}
}

func TestBuildURL(t *testing.T) {
	client := NewClient("test-key", "")
	client.baseURL = "https://api.example.com"
	got, err := client.BuildURL("/v1/path", map[string]string{"p": "v"}, map[string][]string{"a": {"x"}})
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	if strings.Contains(got, "api_key") || strings.Contains(got, "test-key") {
		t.Errorf("URL %q must not contain the API key", got)
	}
	if !strings.Contains(got, "p=v") {
		t.Errorf("URL %q missing p=v", got)
	}
	if !strings.Contains(got, "a=x") {
		t.Errorf("URL %q missing a=x", got)
	}
}

func TestListAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataAssets)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	assets, err := client.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1", len(assets))
	}
	if assets[0].ID != "BTC" || assets[0].Symbol != "BTC" || assets[0].Name != "Bitcoin" {
		t.Errorf("got asset %+v", assets[0])
	}
}

func TestListAssets_WithFilter(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataAssets)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	filter := "asset.semantic_tags.exists(tag,tag=='stablecoin')"
	_, err := client.ListAssets(context.Background(), filter)
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if !strings.Contains(capturedURL, "filter=") {
		t.Errorf("URL %q missing filter param", capturedURL)
	}
	if !strings.Contains(capturedURL, "stablecoin") {
		t.Errorf("URL %q should contain filter value", capturedURL)
	}
}

func TestListMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricsList)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	metrics, err := client.ListMetrics(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	want := []string{"/market/price_usd_close", "/addresses/active_count"}
	if len(metrics) != len(want) {
		t.Fatalf("got %d metrics, want %d", len(metrics), len(want))
	}
	for i, m := range want {
		if metrics[i] != m {
			t.Errorf("metrics[%d] = %q, want %q", i, metrics[i], m)
		}
	}
}

func TestListMetrics_WithAsset(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricsList)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.ListMetrics(context.Background(), map[string]string{"a": "BTC"}, nil)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if !strings.Contains(capturedURL, "a=BTC") {
		t.Errorf("URL %q missing a=BTC param", capturedURL)
	}
}

func TestDescribeMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricDescribe)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	meta, err := client.DescribeMetric(context.Background(), "/market/price_usd_close", "")
	if err != nil {
		t.Fatalf("DescribeMetric: %v", err)
	}
	if meta.Path != "/market/marketcap_usd" {
		t.Errorf("got Path %q, want /market/marketcap_usd", meta.Path)
	}
	if !meta.BulkSupported {
		t.Error("expected BulkSupported true")
	}
}

func TestDescribeMetric_WithAsset(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricDescribe)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.DescribeMetric(context.Background(), "/market/price_usd_close", "BTC")
	if err != nil {
		t.Fatalf("DescribeMetric: %v", err)
	}
	if !strings.Contains(capturedURL, "a=BTC") {
		t.Errorf("URL %q missing a=BTC param", capturedURL)
	}
}

func TestNormalizePath_AddsLeadingSlash(t *testing.T) {
	got := NormalizePath("market/price")
	if got != "/market/price" {
		t.Errorf("got %q, want /market/price", got)
	}
}

func TestNormalizePath_KeepsExistingSlash(t *testing.T) {
	got := NormalizePath("/market/price")
	if got != "/market/price" {
		t.Errorf("got %q, want /market/price", got)
	}
}

func TestGetMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricPoints)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	points, err := client.GetMetric(context.Background(), "/market/price_usd_close", nil)
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("got %d points, want 3", len(points))
	}
	if points[0].T != 1230940800 {
		t.Errorf("got T %d, want 1230940800", points[0].T)
	}
	if points[0].V != 2.4755000000000003 {
		t.Errorf("got V %v, want 2.4755000000000003", points[0].V)
	}
}

func TestGetMetricBulk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataMetricBulk)
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	resp, err := client.GetMetricBulk(context.Background(), "/market/price_usd_close", map[string]string{"s": "1770076800"}, nil)
	if err != nil {
		t.Fatalf("GetMetricBulk: %v", err)
	}
	if len(resp.Data) != 31 {
		t.Fatalf("got %d data points, want 31", len(resp.Data))
	}
	if resp.Data[0].T != 1770076800 {
		t.Errorf("got T %d, want 1770076800", resp.Data[0].T)
	}
	if len(resp.Data[0].Bulk) != 2 {
		t.Fatalf("got %d bulk entries, want 2", len(resp.Data[0].Bulk))
	}
	if resp.Data[0].Bulk[0]["a"] != "BTC" {
		t.Errorf("got a=%v", resp.Data[0].Bulk[0]["a"])
	}
	if resp.Data[0].Bulk[0]["v"] != 1513125006987.5164 {
		t.Errorf("got v=%v", resp.Data[0].Bulk[0]["v"])
	}
}

// Invalid or empty JSON response tests

func TestListAssets_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.ListAssets(context.Background(), "")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestListMetrics_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.ListMetrics(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetMetric_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.GetMetric(context.Background(), "/market/price", nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetMetricBulk_InvalidJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.GetMetricBulk(context.Background(), "/market/price", map[string]string{"s": "1"}, nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// Empty API response tests (point 7)

func TestListAssets_EmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	assets, err := client.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("got %d assets, want 0", len(assets))
	}
}

func TestListMetrics_EmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	metrics, err := client.ListMetrics(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics, want 0", len(metrics))
	}
}

func TestGetMetric_EmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	points, err := client.GetMetric(context.Background(), "/market/price", nil)
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("got %d points, want 0", len(points))
	}
}

func TestGetMetricBulk_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	resp, err := client.GetMetricBulk(context.Background(), "/market/price", map[string]string{"s": "1"}, nil)
	if err != nil {
		t.Fatalf("GetMetricBulk: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("got %d data points, want 0", len(resp.Data))
	}
}

// 4xx with body (point 7)

func TestDo_4xxReturnsErrorWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
	}))
	defer server.Close()

	client := NewClient("test-api-secret", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, err := client.Do(context.Background(), "GET", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error for 401 status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401: %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error should include response body: %v", err)
	}
}

func TestGetMetricBulk_KeepsAllSelectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"t":1,"bulk":[
			{"a":"BTC","e":"binance","v":1},
			{"a":"ETH","category":"more_10y","v":null}]}]}`))
	}))
	defer server.Close()

	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	resp, err := client.GetMetricBulk(context.Background(), "/distribution/balance_exchanges", map[string]string{"s": "1"}, nil)
	if err != nil {
		t.Fatalf("GetMetricBulk: %v", err)
	}
	rows := resp.Data[0].Bulk
	if rows[0]["e"] != "binance" || rows[0]["a"] != "BTC" || rows[0]["v"] != 1.0 {
		t.Errorf("exchange selector lost: %v", rows[0])
	}
	if rows[1]["category"] != "more_10y" || rows[1]["v"] != nil {
		t.Errorf("category selector lost: %v", rows[1])
	}
}

func TestMetricParamsMapsTypedFields(t *testing.T) {
	p, err := metricParams(map[string]string{"s": "100", "u": "200", "i": "24h", "c": "USD", "a": "BTC", "network": "eth"}, map[string][]string{"e": {"binance", "coinbase"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Since.Unix() != 100 || p.Until.Unix() != 200 || p.Interval != "24h" || p.Currency != "USD" || p.Asset != "BTC" || len(p.Exchanges) != 2 {
		t.Errorf("typed fields: %+v", p)
	}
	if p.Extra.Get("network") != "eth" || p.Extra.Has("s") || p.Extra.Has("a") || p.Extra.Has("e") {
		t.Errorf("extra: %v", p.Extra)
	}
	if _, err := metricParams(map[string]string{"s": "yesterday"}, nil); err == nil {
		t.Error("accepted non-numeric since")
	}
}
