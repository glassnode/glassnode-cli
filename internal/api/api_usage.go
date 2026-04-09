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
func (a *APIUsageResponse) CreditsPerMonth() int {
	var max int
	for _, a := range a.APIAddons {
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

func (a *APIUsageResponse) Summary() CreditsSummary {
	per := a.CreditsPerMonth()
	left := per - a.CreditsUsed
	if left < 0 {
		left = 0
	}

	return CreditsSummary{
		CreditsUsed:     a.CreditsUsed,
		CreditsPerMonth: per,
		CreditsLeft:     left,
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
