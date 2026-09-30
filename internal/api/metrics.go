package api

import (
	"context"
	"encoding/json"
	"fmt"

	glassnode "github.com/glassnode/glassnode-api-go-client"
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

// UnmarshalJSON keeps the CLI's historical dynamic number/output behavior.
func (p *DataPoint) UnmarshalJSON(data []byte) error {
	type wire DataPoint
	return json.Unmarshal(data, (*wire)(p))
}

func (c *Client) GetMetric(ctx context.Context, path string, params map[string]string) ([]DataPoint, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	var points []DataPoint
	if err := client.GetMetric(ctx, path, &glassnode.MetricParams{Extra: queryValues(params, nil)}, &points); err != nil {
		return nil, fmt.Errorf("getting metric: %w", err)
	}
	return points, nil
}

func (c *Client) GetMetricBulk(ctx context.Context, path string, params map[string]string, repeated map[string][]string) (*BulkResponse, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	points, err := client.GetBulkMetric(ctx, path, &glassnode.MetricParams{Extra: queryValues(params, repeated)})
	if err != nil {
		return nil, fmt.Errorf("getting bulk metric: %w", err)
	}
	response := &BulkResponse{Data: make([]BulkDataPoint, 0, len(points))}
	for _, point := range points {
		entries := make([]map[string]interface{}, 0, len(point.Bulk))
		for _, entry := range point.Bulk {
			var value any
			if entry.Value != nil {
				value = *entry.Value
			}
			row := map[string]interface{}{"a": entry.Asset, "v": value}
			if entry.Network != "" {
				row["network"] = entry.Network
			}
			entries = append(entries, row)
		}
		response.Data = append(response.Data, BulkDataPoint{T: point.Timestamp, Bulk: entries})
	}
	return response, nil
}
