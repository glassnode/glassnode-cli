package api

import (
	"context"
	"fmt"
	"strings"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

// DataPoint is a metric point as the API sent it. Numbers are json.Number,
// since the SDK decodes with UseNumber: -o json prints them unchanged and
// integers above 2^53 keep their precision.
type DataPoint struct {
	T int64                  `json:"t"`
	V interface{}            `json:"v,omitempty"`
	O map[string]interface{} `json:"o,omitempty"`
	// ComputedAt is set by point-in-time metrics: when the value was computed.
	ComputedAt *int64 `json:"computed_at,omitempty"`
}

type BulkDataPoint struct {
	T    int64                    `json:"t"`
	Bulk []map[string]interface{} `json:"bulk"`
}

type BulkResponse struct {
	Data []BulkDataPoint `json:"data"`
}

func (c *Client) GetMetric(ctx context.Context, path string, params map[string]string) ([]DataPoint, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	// The SDK reads s, u, i, c, a and e from Extra and validates them.
	p := &glassnode.MetricParams{Extra: queryValues(params, nil)}
	var points []DataPoint
	if err := client.GetMetric(ctx, path, p, &points); err != nil {
		return nil, fmt.Errorf("getting metric: %w", err)
	}
	return points, nil
}

// GetMetricBulk fetches the bulk variant of a metric and returns its entries as
// the API sent them. The CLI only prints them, so it decodes into its own rows
// rather than the SDK's typed BulkEntry: numbers stay numbers (the SDK decodes
// with UseNumber), large integers keep their precision and selectors or values
// of any type pass through.
func (c *Client) GetMetricBulk(ctx context.Context, path string, params map[string]string, repeated map[string][]string) (*BulkResponse, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	p := &glassnode.MetricParams{Extra: queryValues(params, repeated)}
	var response BulkResponse
	if err := client.GetMetric(ctx, strings.TrimSuffix(NormalizePath(path), "/")+"/bulk", p, &response); err != nil {
		return nil, fmt.Errorf("getting bulk metric: %w", err)
	}
	if response.Data == nil {
		response.Data = []BulkDataPoint{}
	}
	return &response, nil
}
