package api

import (
	"context"
	"net/url"
	"strings"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

type Asset = glassnode.Asset
type Blockchain = glassnode.Blockchain
type AssetsResponse = glassnode.AssetsResponse
type NamesResponse = glassnode.NamesResponse
type NameEntry = glassnode.NameEntry
type MetricMetadata = glassnode.MetricMetadata
type MetricVariant = glassnode.MetricVariant
type MetricDescriptors = glassnode.MetricDescriptors
type Timerange = glassnode.TimeRange
type Refs = glassnode.Refs

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
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	return client.ListAssets(ctx, filter)
}

func (c *Client) ListNames(ctx context.Context, path, filter string) ([]string, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	return client.ListNames(ctx, path, filter)
}

func (c *Client) ListMetrics(ctx context.Context, params map[string]string, repeated map[string][]string) ([]string, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	return client.ListMetrics(ctx, &glassnode.MetricParams{Extra: queryValues(params, repeated)})
}

func (c *Client) DescribeMetric(ctx context.Context, path, asset string) (*MetricMetadata, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	return client.GetMetricMetadata(ctx, path, &glassnode.MetricParams{Asset: asset})
}

// BuildURL constructs the request URL without executing the request. The
// credential is not part of it: it travels in the header named by AuthHeader.
func (c *Client) BuildURL(path string, params map[string]string, repeatedParams map[string][]string) (string, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	u.RawQuery = queryValues(params, repeatedParams).Encode()
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
