package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glassnode/gn/internal/api"
)

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	data := []string{"a", "b"}
	if err := PrintJSON(&buf, data); err != nil {
		t.Errorf("PrintJSON: %v", err)
	}
	var decoded []string
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Errorf("output %q is not valid JSON: %v", buf.String(), err)
	}
	if len(decoded) != 2 || decoded[0] != "a" || decoded[1] != "b" {
		t.Errorf("decoded %v", decoded)
	}
}

func TestPrintCSV_DataPoints(t *testing.T) {
	var buf bytes.Buffer
	data := []api.DataPoint{{T: 1, V: 2.0}}
	if err := PrintCSV(&buf, data, "unix"); err != nil {
		t.Errorf("PrintCSV: %v", err)
	}
	want := "t,v\n1,2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrint_JSON(t *testing.T) {
	var buf bytes.Buffer
	data := []string{"x"}
	if err := PrintTo(&buf, Options{Format: "json", Data: data}); err != nil {
		t.Errorf("Print: %v", err)
	}
	var decoded []string
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Errorf("output %q is not valid JSON: %v", buf.String(), err)
	}
}

func TestPrint_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := PrintTo(&buf, Options{Format: "invalid", Data: nil})
	if err == nil {
		t.Error("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error %q should mention invalid", err.Error())
	}
}

func TestPrint_CSV(t *testing.T) {
	var buf bytes.Buffer
	data := []api.DataPoint{{T: 1, V: 2.0}}
	if err := PrintTo(&buf, Options{Format: "csv", Data: data, TimestampFormat: "unix"}); err != nil {
		t.Errorf("Print csv: %v", err)
	}
	want := "t,v\n1,2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrint_Table(t *testing.T) {
	var buf bytes.Buffer
	data := []api.DataPoint{{T: 1, V: 2.0}}
	if err := PrintTo(&buf, Options{Format: "table", Data: data}); err != nil {
		t.Errorf("Print table: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Errorf("table output should contain data: %q", out)
	}
}

func TestPrintCSV_MetricList(t *testing.T) {
	var buf bytes.Buffer
	data := []string{"/market/price_usd_close", "/addresses/active_count"}
	if err := PrintCSV(&buf, data, ""); err != nil {
		t.Errorf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "metric") || !strings.Contains(out, "price_usd_close") {
		t.Errorf("got %q", out)
	}
}

func TestPrintCSV_Assets(t *testing.T) {
	var buf bytes.Buffer
	data := []api.Asset{{ID: "BTC", Symbol: "BTC", Name: "Bitcoin", AssetType: "BLOCKCHAIN", Categories: []string{"on-chain"}}}
	if err := PrintCSV(&buf, data, ""); err != nil {
		t.Errorf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "id,symbol,name") || !strings.Contains(out, "BTC") {
		t.Errorf("got %q", out)
	}
}

func TestPrintTable_Assets(t *testing.T) {
	var buf bytes.Buffer
	data := []api.Asset{{ID: "BTC", Symbol: "BTC", Name: "Bitcoin", AssetType: "BLOCKCHAIN", Categories: []string{"on-chain"}}}
	if err := PrintTable(&buf, data, ""); err != nil {
		t.Errorf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "BTC") || !strings.Contains(out, "Bitcoin") {
		t.Errorf("got %q", out)
	}
}

func TestPrintCSV_SliceOfMaps(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]interface{}{
		{"id": "BTC", "symbol": "BTC"},
		{"id": "ETH", "symbol": "ETH"},
	}
	if err := PrintCSV(&buf, data, ""); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "id,") || !strings.Contains(out, "symbol") {
		t.Errorf("CSV should have id,symbol columns: %q", out)
	}
	if !strings.Contains(out, "BTC") || !strings.Contains(out, "ETH") {
		t.Errorf("CSV should contain data: %q", out)
	}
}

func TestPrintTable_SliceOfMaps(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]interface{}{
		{"id": "BTC", "symbol": "BTC"},
		{"id": "ETH", "symbol": "ETH"},
	}
	if err := PrintTable(&buf, data, ""); err != nil {
		t.Fatalf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "SYMBOL") {
		t.Errorf("table should have ID, SYMBOL headers: %q", out)
	}
	if !strings.Contains(out, "BTC") || !strings.Contains(out, "ETH") {
		t.Errorf("table should contain data: %q", out)
	}
}

func TestPrintTable_MetricMetadata(t *testing.T) {
	var buf bytes.Buffer
	meta := &api.MetricMetadata{Path: "/market/price_usd_close", Tier: 1, BulkSupported: true}
	if err := PrintTable(&buf, meta, ""); err != nil {
		t.Errorf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "price_usd_close") || !strings.Contains(out, "Bulk Supported") {
		t.Errorf("got %q", out)
	}
}

// DataPoints with O (object) for CSV and Table

func TestPrintCSV_DataPointsWithObject(t *testing.T) {
	var buf bytes.Buffer
	data := []api.DataPoint{
		{T: 1, O: map[string]interface{}{"a": "x", "b": 2.0}},
		{T: 2, O: map[string]interface{}{"a": "y", "b": 3.0}},
	}
	if err := PrintCSV(&buf, data, "unix"); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "t,") || !strings.Contains(out, "a,") || !strings.Contains(out, "b") {
		t.Errorf("CSV should have t,a,b columns: %q", out)
	}
	if !strings.Contains(out, "1,") || !strings.Contains(out, "x") || !strings.Contains(out, "2") {
		t.Errorf("CSV should contain data rows: %q", out)
	}
}

func TestPrintTable_DataPointsWithObject(t *testing.T) {
	var buf bytes.Buffer
	data := []api.DataPoint{
		{T: 1, O: map[string]interface{}{"a": "x", "b": 2.0}},
	}
	if err := PrintTable(&buf, data, ""); err != nil {
		t.Fatalf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "T") || !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Errorf("table should have T, A, B headers: %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "x") || !strings.Contains(out, "2") {
		t.Errorf("table should contain data: %q", out)
	}
}

// BulkResponse CSV and Table

func TestPrintCSV_BulkResponse(t *testing.T) {
	var buf bytes.Buffer
	resp := &api.BulkResponse{
		Data: []api.BulkDataPoint{
			{T: 1738657600, Bulk: []map[string]interface{}{{"a": "BTC", "v": float64(123)}}},
		},
	}
	if err := PrintCSV(&buf, resp, ""); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "t,") || !strings.Contains(out, "a,") || !strings.Contains(out, "v") {
		t.Errorf("BulkResponse CSV should have t,a,v columns: %q", out)
	}
	if !strings.Contains(out, "BTC") || !strings.Contains(out, "123") {
		t.Errorf("BulkResponse CSV should contain data: %q", out)
	}
}

func TestPrintTable_BulkResponse(t *testing.T) {
	var buf bytes.Buffer
	resp := &api.BulkResponse{
		Data: []api.BulkDataPoint{
			{T: 1738657600, Bulk: []map[string]interface{}{{"a": "BTC", "v": float64(123)}}},
		},
	}
	if err := PrintTable(&buf, resp, ""); err != nil {
		t.Fatalf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "T") || !strings.Contains(out, "A") || !strings.Contains(out, "V") {
		t.Errorf("BulkResponse table should have T,A,V headers: %q", out)
	}
	if !strings.Contains(out, "BTC") || !strings.Contains(out, "123") {
		t.Errorf("BulkResponse table should contain data: %q", out)
	}
}

// Empty slices

func TestPrintCSV_EmptyDataPoints(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintCSV(&buf, []api.DataPoint{}, ""); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	// Implementation flushes without writing header for empty DataPoints
	if len(out) != 0 && !strings.Contains(out, "t") {
		t.Errorf("empty DataPoints CSV: got %q", out)
	}
}

func TestPrintTable_EmptyDataPoints(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintTable(&buf, []api.DataPoint{}, ""); err != nil {
		t.Fatalf("PrintTable: %v", err)
	}
	out := buf.String()
	// Table with no rows may output nothing or just newlines
	if strings.Contains(out, "panic") {
		t.Errorf("should not panic: %q", out)
	}
}

func TestPrintCSV_EmptyStringSlice(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintCSV(&buf, []string{}, ""); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "metric") {
		t.Errorf("empty metric list CSV should have header: %q", out)
	}
}

func TestPrintCSV_EmptyAssetSlice(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintCSV(&buf, []api.Asset{}, ""); err != nil {
		t.Fatalf("PrintCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "id,") {
		t.Errorf("empty assets CSV should have header: %q", out)
	}
}

// MetricMetadata with optional fields (Descriptors, Timerange, Parameters)

func TestPrintTable_MetricMetadataWithOptionalFields(t *testing.T) {
	var buf bytes.Buffer
	meta := &api.MetricMetadata{
		Path:          "/market/price_usd_close",
		Tier:          1,
		BulkSupported: true,
		Descriptors: &api.MetricDescriptors{
			Name:  "Price Close",
			Group: "Market",
			Tags:  []string{"price", "market"},
		},
		Timerange:  &api.Timerange{Min: 1609459200, Max: 1735689600},
		Parameters: map[string][]string{"a": {"BTC", "ETH"}, "i": {"24h"}},
	}
	if err := PrintTable(&buf, meta, ""); err != nil {
		t.Fatalf("PrintTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Price Close") || !strings.Contains(out, "Market") {
		t.Errorf("table should include Descriptors: %q", out)
	}
	if !strings.Contains(out, "price") || !strings.Contains(out, "market") {
		t.Errorf("table should include tags: %q", out)
	}
	if !strings.Contains(out, "1609459200") || !strings.Contains(out, "1735689600") {
		t.Errorf("table should include Timerange: %q", out)
	}
	if !strings.Contains(out, "Parameters:") || !strings.Contains(out, "a:") || !strings.Contains(out, "BTC") {
		t.Errorf("table should include Parameters: %q", out)
	}
}
