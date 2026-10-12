package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// metricParams maps the CLI's flat query parameters onto the SDK's typed
// fields, so the SDK validates the time range and selectors; anything it has
// no field for stays in Extra.
func metricParams(params map[string]string, repeated map[string][]string) (*glassnode.MetricParams, error) {
	p := &glassnode.MetricParams{Extra: queryValues(params, repeated)}
	for key, value := range params {
		switch key {
		case "s", "u":
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parameter %s: expected Unix seconds, got %q", key, value)
			}
			if key == "s" {
				p.Since = time.Unix(seconds, 0)
			} else {
				p.Until = time.Unix(seconds, 0)
			}
		case "i":
			p.Interval = value
		case "c":
			p.Currency = value
		case "a":
			p.Asset = value
		case "e":
			p.Exchanges = []string{value}
		default:
			continue
		}
		p.Extra.Del(key)
	}
	if assets := repeated["a"]; len(assets) > 0 {
		p.Assets = assets
		p.Extra.Del("a")
	}
	if exchanges := repeated["e"]; len(exchanges) > 0 {
		p.Exchanges = exchanges
		p.Extra.Del("e")
	}
	return p, nil
}

func (c *Client) GetMetric(ctx context.Context, path string, params map[string]string) ([]DataPoint, error) {
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	p, err := metricParams(params, nil)
	if err != nil {
		return nil, err
	}
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
	p, err := metricParams(params, repeated)
	if err != nil {
		return nil, err
	}
	var response BulkResponse
	if err := client.GetMetric(ctx, strings.TrimSuffix(NormalizePath(path), "/")+"/bulk", p, &response); err != nil {
		return nil, fmt.Errorf("getting bulk metric: %w", err)
	}
	if response.Data == nil {
		response.Data = []BulkDataPoint{}
	}
	return &response, nil
}
