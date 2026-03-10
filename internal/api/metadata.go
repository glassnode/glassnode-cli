package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Asset struct {
	ID             string            `json:"id"`
	ExternalIDs    map[string]string `json:"external_ids,omitempty"`
	Symbol         string            `json:"symbol"`
	Name           string            `json:"name"`
	AssetType      string            `json:"asset_type"`
	Blockchains    []Blockchain      `json:"blockchains"`
	Categories     []string          `json:"categories,omitempty"`
	LogoURL        string            `json:"logo_url,omitempty"`
	SemanticTags   []string          `json:"semantic_tags,omitempty"`
	DefaultNetwork string            `json:"default_network,omitempty"`
}

type Blockchain struct {
	Blockchain     string `json:"blockchain"`
	Address        string `json:"address,omitempty"`
	Decimals       int    `json:"decimals,omitempty"`
	OnChainSupport bool   `json:"on_chain_support"`
}

type AssetsResponse struct {
	Data []Asset `json:"data"`
}

type MetricMetadata struct {
	Path          string              `json:"path,omitempty"`
	Tier          float64             `json:"tier,omitempty"`
	IsPit         bool                `json:"is_pit,omitempty"`
	Parameters    map[string][]string `json:"parameters"`
	Queried       map[string]string   `json:"queried,omitempty"`
	Refs          Refs                `json:"refs,omitempty"`
	BulkSupported bool                `json:"bulk_supported"`
	Timerange     *Timerange          `json:"timerange,omitempty"`
	Modified      int64               `json:"modified,omitempty"`
	Descriptors   *MetricDescriptors  `json:"descriptors,omitempty"`
}

type MetricVariant struct {
	Base *string `json:"base,omitempty"`
	Bulk *string `json:"bulk,omitempty"`
	Pit  *string `json:"pit,omitempty"`
}

type MetricDescriptors struct {
	Name             string            `json:"name,omitempty"`
	ShortName        string            `json:"short_name,omitempty"`
	Group            string            `json:"group,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Description      map[string]string `json:"description,omitempty"`
	DataSharingGroup string            `json:"data_sharing_group,omitempty"`
}

type Timerange struct {
	Min int64 `json:"min,omitempty"`
	Max int64 `json:"max,omitempty"`
}

type Refs struct {
	Doc           string         `json:"docs,omitempty"`
	Studio        string         `json:"studio,omitempty"`
	MetricVariant *MetricVariant `json:"metric_variant,omitempty"`
}

// assetToMap returns a map of JSON field names to values for the given asset.
// Used by PruneAssets to build objects with only requested fields.
func assetToMap(a Asset) map[string]interface{} {
	m := map[string]interface{}{
		"id":              a.ID,
		"symbol":          a.Symbol,
		"name":            a.Name,
		"asset_type":      a.AssetType,
		"categories":      a.Categories,
		"logo_url":        a.LogoURL,
		"semantic_tags":   a.SemanticTags,
		"default_network": a.DefaultNetwork,
		"external_ids":    a.ExternalIDs,
		"blockchains":     a.Blockchains,
	}
	return m
}

// PruneAssets returns a slice of maps, each containing only the requested fields
// (JSON-style names: id, symbol, name, asset_type, categories, etc.). Use with
// --prune to get an array of objects with a subset of fields.
func PruneAssets(assets []Asset, fields []string) []map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(assets))
	for _, a := range assets {
		full := assetToMap(a)
		pruned := make(map[string]interface{}, len(fields))
		for _, f := range fields {
			k := strings.TrimSpace(f)
			if v, ok := full[k]; ok {
				pruned[k] = v
			}
		}
		out = append(out, pruned)
	}
	return out
}

func (c *Client) ListAssets(ctx context.Context, filter string) ([]Asset, error) {
	params := map[string]string{}
	if filter != "" {
		params["filter"] = filter
	}
	body, err := c.Do(ctx, "GET", "/v1/metadata/assets", params)
	if err != nil {
		return nil, fmt.Errorf("listing assets: %w", err)
	}
	var resp AssetsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding assets response: %w", err)
	}
	return resp.Data, nil
}

// ListMetrics lists available metric paths, optionally filtered by query parameters.
// See https://docs.glassnode.com/basic-api/metadata#query-parameters-1
// params: a (asset), c (currency), e (exchange), f (format), i (interval),
// from_exchange, to_exchange, miner, maturity, network, period, quote_symbol.
// repeatedParams: use key "a" for multiple assets (e.g. a=BTC&a=ETH).
func (c *Client) ListMetrics(ctx context.Context, params map[string]string, repeatedParams map[string][]string) ([]string, error) {
	if params == nil {
		params = map[string]string{}
	}
	if repeatedParams == nil {
		repeatedParams = map[string][]string{}
	}
	body, err := c.DoWithRepeatedParams(ctx, "GET", "/v1/metadata/metrics", params, repeatedParams)
	if err != nil {
		return nil, fmt.Errorf("listing metrics: %w", err)
	}
	var metrics []string
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("decoding metrics response: %w", err)
	}
	return metrics, nil
}

func (c *Client) DescribeMetric(ctx context.Context, path, asset string) (*MetricMetadata, error) {
	params := map[string]string{"path": path}
	if asset != "" {
		params["a"] = asset
	}
	body, err := c.Do(ctx, "GET", "/v1/metadata/metric", params)
	if err != nil {
		return nil, fmt.Errorf("describing metric: %w", err)
	}
	var meta MetricMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("decoding metric metadata: %w", err)
	}
	return &meta, nil
}

// BuildURL constructs the full request URL without executing the request.
func (c *Client) BuildURL(path string, params map[string]string, repeatedParams map[string][]string) (string, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	q := u.Query()
	q.Set("api_key", c.APIKey)
	for k, v := range params {
		q.Set(k, v)
	}
	for k, vals := range repeatedParams {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// RedactAPIKeyFromURL returns a copy of the URL with the api_key query parameter
// replaced by a placeholder, for safe display (e.g. dry-run output).
func RedactAPIKeyFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Has("api_key") {
		q.Set("api_key", "***")
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// NormalizePath ensures the metric path has a leading slash.
func NormalizePath(path string) string {
	return "/" + strings.TrimLeft(path, "/")
}

const bulkPathSuffix = "/bulk"

// IsBulkPath reports whether the normalized path refers to the bulk metric endpoint.
func IsBulkPath(path string) bool {
	return strings.HasSuffix(path, bulkPathSuffix)
}

// TrimBulkSuffix returns path with a trailing "/bulk" removed. Use after NormalizePath when calling the bulk API.
func TrimBulkSuffix(path string) string {
	return strings.TrimSuffix(path, bulkPathSuffix)
}
