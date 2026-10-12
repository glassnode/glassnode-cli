package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPruneAssets(t *testing.T) {
	assets := []Asset{
		{ID: "BTC", Symbol: "BTC", Name: "Bitcoin", AssetType: "BLOCKCHAIN", Categories: []string{"on-chain", "spot"}},
		{ID: "ETH", Symbol: "ETH", Name: "Ethereum", AssetType: "BLOCKCHAIN", Categories: []string{"on-chain"}},
	}

	t.Run("single field", func(t *testing.T) {
		got := PruneAssets(assets, []string{"id"})
		if len(got) != 2 {
			t.Fatalf("got %d objects, want 2", len(got))
		}
		if g, _ := got[0]["id"].(string); g != "BTC" {
			t.Errorf("got[0][id] = %q, want BTC", g)
		}
		if g, _ := got[1]["id"].(string); g != "ETH" {
			t.Errorf("got[1][id] = %q, want ETH", g)
		}
		if _, ok := got[0]["name"]; ok {
			t.Error("pruned object should not contain name")
		}
	})

	t.Run("multiple fields", func(t *testing.T) {
		got := PruneAssets(assets, []string{"id", "symbol", "name"})
		if len(got) != 2 {
			t.Fatalf("got %d objects, want 2", len(got))
		}
		if g, _ := got[0]["id"].(string); g != "BTC" {
			t.Errorf("got[0][id] = %q, want BTC", g)
		}
		if g, _ := got[0]["symbol"].(string); g != "BTC" {
			t.Errorf("got[0][symbol] = %q, want BTC", g)
		}
		if g, _ := got[0]["name"].(string); g != "Bitcoin" {
			t.Errorf("got[0][name] = %q, want Bitcoin", g)
		}
		if _, ok := got[0]["asset_type"]; ok {
			t.Error("pruned object should not contain asset_type")
		}
	})

	t.Run("categories slice", func(t *testing.T) {
		got := PruneAssets(assets, []string{"id", "categories"})
		if len(got) != 2 {
			t.Fatalf("got %d objects, want 2", len(got))
		}
		cats, ok := got[0]["categories"].([]string)
		if !ok {
			t.Errorf("categories should be []string, got %T", got[0]["categories"])
		}
		if len(cats) != 2 || cats[0] != "on-chain" || cats[1] != "spot" {
			t.Errorf("got categories %v", cats)
		}
	})

	t.Run("empty fields returns nil", func(t *testing.T) {
		got := PruneAssets(assets, nil)
		if got != nil {
			t.Errorf("PruneAssets(_, nil) = %v, want nil", got)
		}
		got = PruneAssets(assets, []string{})
		if got != nil {
			t.Errorf("PruneAssets(_, []) = %v, want nil", got)
		}
	})

	t.Run("unknown field omitted", func(t *testing.T) {
		got := PruneAssets(assets, []string{"id", "nonexistent"})
		if len(got) != 2 {
			t.Fatalf("got %d objects, want 2", len(got))
		}
		if _, ok := got[0]["nonexistent"]; ok {
			t.Error("unknown field should not appear")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		got := PruneAssets(assets, []string{"id", "symbol"})
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded []map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(decoded) != 2 {
			t.Fatalf("decoded %d objects, want 2", len(decoded))
		}
		if decoded[0]["id"] != "BTC" || decoded[0]["symbol"] != "BTC" {
			t.Errorf("decoded[0] = %v", decoded[0])
		}
	})
}

func TestIsBulkPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/market/marketcap_usd/bulk", true},
		{"market/marketcap_usd/bulk", true},
		{"/market/price", false},
		{"/market/price/", false},
		{"/bulk", true},
		{"", false},
	}
	for _, tt := range tests {
		p := NormalizePath(tt.path)
		got := IsBulkPath(p)
		if got != tt.want {
			t.Errorf("IsBulkPath(NormalizePath(%q)) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestTrimBulkSuffix(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/market/marketcap_usd/bulk", "/market/marketcap_usd"},
		{"/market/price", "/market/price"},
		{"/bulk", ""},
	}
	for _, tt := range tests {
		got := TrimBulkSuffix(tt.path)
		if got != tt.want {
			t.Errorf("TrimBulkSuffix(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFilterMetricsByFormat(t *testing.T) {
	formats := map[string][]string{
		"/market/price": {"csv", "json"}, "/derivatives/term_structure": {"json"},
		"/market/hodl_cave": {"CSV", "json"}, "/indicators/urpd": {"json"},
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		path := r.URL.Query().Get("path")
		f, ok := formats[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "metadata not found: path=%s", path)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"path": path, "parameters": map[string]any{"f": f, "a": []string{"BTC"}}})
	}))
	defer server.Close()
	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	paths := []string{"/market/price", "/derivatives/term_structure", "/options/smile", "/market/hodl_cave", "/indicators/urpd"}
	kept, skipped, err := client.FilterMetricsByFormat(context.Background(), paths, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/market/price", "/market/hodl_cave"}; !slices.Equal(kept, want) {
		t.Errorf("kept = %v, want %v (in list order)", kept, want)
	}
	if want := []string{"/options/smile"}; !slices.Equal(skipped, want) {
		t.Errorf("skipped = %v, want %v", skipped, want)
	}
	if calls.Load() != int32(len(paths)) {
		t.Errorf("metadata calls = %d, want %d", calls.Load(), len(paths))
	}
	kept, _, err = client.FilterMetricsByFormat(context.Background(), paths, "json")
	if err != nil || len(kept) != 4 {
		t.Errorf("json: kept=%v err=%v", kept, err)
	}
}

func TestFilterMetricsByFormat_AbortsOnOtherErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"no access"}`)
	}))
	defer server.Close()
	client := NewClient("key", "")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	if _, _, err := client.FilterMetricsByFormat(context.Background(), []string{"/a", "/b"}, "csv"); err == nil || !strings.Contains(err.Error(), "no access") {
		t.Fatalf("err = %v, want the API error", err)
	}
}
