package api

import (
	"context"
	"encoding/json"
	"fmt"
)

type DataPoint struct {
	T int64                  `json:"t"`
	V interface{}            `json:"v,omitempty"`
	O map[string]interface{} `json:"o,omitempty"`
}

type BulkDataPoint struct {
	T    int64                    `json:"t"`
	Bulk []map[string]interface{} `json:"bulk"`
}

type BulkResponse struct {
	Data []BulkDataPoint `json:"data"`
}

func (c *Client) GetMetric(ctx context.Context, path string, params map[string]string) ([]DataPoint, error) {
	normalized := NormalizePath(path)
	body, err := c.Do(ctx, "GET", "/v1/metrics"+normalized, params)
	if err != nil {
		return nil, fmt.Errorf("getting metric: %w", err)
	}
	var points []DataPoint
	if err := json.Unmarshal(body, &points); err != nil {
		return nil, fmt.Errorf("decoding metric response: %w", err)
	}
	return points, nil
}

func (c *Client) GetMetricBulk(ctx context.Context, path string, params map[string]string, repeatedParams map[string][]string) (*BulkResponse, error) {
	normalized := NormalizePath(path)
	body, err := c.DoWithRepeatedParams(ctx, "GET", "/v1/metrics"+normalized+"/bulk", params, repeatedParams)
	if err != nil {
		return nil, fmt.Errorf("getting bulk metric: %w", err)
	}
	var resp BulkResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding bulk metric response: %w", err)
	}
	return &resp, nil
}
