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

// CreditsLimit returns the largest addon credit value, or 0 when there are no addons
func (a *APIUsageResponse) CreditsLimit() int {
	var max int
	for _, a := range a.APIAddons {
		if a.Value > max {
			max = a.Value
		}
	}

	return max
}

// CreditsSummary is the CLI response to the end user. CreditsLimit is the
// allowance granted per CreditsPeriod, which depends on the account's products.
type CreditsSummary struct {
	CreditsLeft   int           `json:"creditsLeft"`
	CreditsLimit  int           `json:"creditsLimit"`
	CreditsPeriod CreditsPeriod `json:"creditsPeriod"`
	CreditsUsed   int           `json:"creditsUsed"`
}

// Summary builds the credits summary for an allowance granted over period.
// Use UserInfoResponse.CreditsPeriod to resolve the period for an account.
func (a *APIUsageResponse) Summary(period CreditsPeriod) CreditsSummary {
	limit := a.CreditsLimit()
	left := limit - a.CreditsUsed
	if left < 0 {
		left = 0
	}

	return CreditsSummary{
		CreditsUsed:   a.CreditsUsed,
		CreditsLimit:  limit,
		CreditsPeriod: period,
		CreditsLeft:   left,
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
