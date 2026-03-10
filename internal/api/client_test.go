package api

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/glassnode/glassnode-cli/internal/config"
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

func TestDo_SendsCorrectURL(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient("my-api-key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	_, err := client.Do(context.Background(), "GET", "/v1/test", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(capturedURL, "api_key=my-api-key") {
		t.Errorf("URL %q missing api_key param", capturedURL)
	}
	if !strings.Contains(capturedURL, "a=b") {
		t.Errorf("URL %q missing a=b param", capturedURL)
	}
}

func TestDo_Non2xxReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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
	client := NewClient("test-key")
	client.BaseURL = "https://api.example.com"
	got, err := client.BuildURL("/v1/path", map[string]string{"p": "v"}, map[string][]string{"a": {"x"}})
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	if !strings.Contains(got, "api_key=test-key") {
		t.Errorf("URL %q missing api_key", got)
	}
	if !strings.Contains(got, "p=v") {
		t.Errorf("URL %q missing p=v", got)
	}
	if !strings.Contains(got, "a=x") {
		t.Errorf("URL %q missing a=x", got)
	}
}

func TestRedactAPIKeyFromURL(t *testing.T) {
	raw := "https://api.example.com/v1/path?api_key=secret123&a=b"
	redacted, err := RedactAPIKeyFromURL(raw)
	if err != nil {
		t.Fatalf("RedactAPIKeyFromURL: %v", err)
	}
	if strings.Contains(redacted, "secret123") {
		t.Errorf("redacted URL should not contain secret: %q", redacted)
	}
	// Placeholder may be URL-encoded as %2A%2A%2A
	if !strings.Contains(redacted, "api_key=***") && !strings.Contains(redacted, "api_key=%2A%2A%2A") {
		t.Errorf("redacted URL should contain api_key redaction: %q", redacted)
	}
	if !strings.Contains(redacted, "a=b") {
		t.Errorf("redacted URL should preserve other params: %q", redacted)
	}
}

func TestListAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(testdataAssets)
	}))
	defer server.Close()

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	resp, err := client.GetMetricBulk(context.Background(), "/market/price_usd_close", nil, nil)
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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	_, err := client.GetMetricBulk(context.Background(), "/market/price", nil, nil)
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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	resp, err := client.GetMetricBulk(context.Background(), "/market/price", nil, nil)
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

	client := NewClient("key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

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

// RedactAPIKeyFromURL edge cases (point 7)

func TestRedactAPIKeyFromURL_NoAPIKeyParam(t *testing.T) {
	raw := "https://api.example.com/v1/path?a=b"
	redacted, err := RedactAPIKeyFromURL(raw)
	if err != nil {
		t.Fatalf("RedactAPIKeyFromURL: %v", err)
	}
	if redacted != raw {
		t.Errorf("URL without api_key should be unchanged: got %q", redacted)
	}
}

func TestRedactAPIKeyFromURL_EmptyAPIKey(t *testing.T) {
	raw := "https://api.example.com/v1/path?api_key=&a=b"
	redacted, err := RedactAPIKeyFromURL(raw)
	if err != nil {
		t.Fatalf("RedactAPIKeyFromURL: %v", err)
	}
	if !strings.Contains(redacted, "a=b") {
		t.Errorf("should preserve other params: %q", redacted)
	}
	if !strings.Contains(redacted, "api_key=") {
		t.Errorf("should still have api_key param: %q", redacted)
	}
}

func TestRedactAPIKeyFromURL_InvalidURL(t *testing.T) {
	_, err := RedactAPIKeyFromURL("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
