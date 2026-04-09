package api

import (
	"context"
	"encoding/json"
	"fmt"
)

type APIAddon struct {
	Value int `json:"value"`
}

type APIUsageResponse struct {
	CreditsUsed int        `json:"creditsUsed"`
	APIAddons   []APIAddon `json:"apiAddons"`
}

// CreditsPerMonth returns the largest addon credit value, or 0 when there are no addons
func (r *APIUsageResponse) CreditsPerMonth() int {
	var max int
	for _, a := range r.APIAddons {
		if a.Value > max {
			max = a.Value
		}
	}

	return max
}

// CreditsSummary is the CLI response to the end user
type CreditsSummary struct {
	CreditsLeft     int `json:"creditsLeft"`
	CreditsPerMonth int `json:"creditsPerMonth"`
	CreditsUsed     int `json:"creditsUsed"`
}

func (r *APIUsageResponse) Summary() CreditsSummary {
	per := r.CreditsPerMonth()
	return CreditsSummary{
		CreditsUsed:     r.CreditsUsed,
		CreditsPerMonth: per,
		CreditsLeft:     per - r.CreditsUsed,
	}
}

// GetAPIUsage fetches the current API usage for the authenticated user
func (c *Client) GetAPIUsage(ctx context.Context) (*APIUsageResponse, error) {
	body, err := c.Do(ctx, "GET", "/v1/user/api_usage", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching API usage: %w", err)
	}

	var out APIUsageResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding API usage response: %w", err)
	}

	return &out, nil
}
