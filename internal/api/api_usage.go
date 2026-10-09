package api

import (
	"context"
	"fmt"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

type APIAddon = glassnode.APIAddon

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
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	usage, err := client.GetAPIUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching API usage response: %w", err)
	}
	return &APIUsageResponse{CreditsUsed: usage.CreditsUsed, APIAddons: usage.APIAddons}, nil
}
