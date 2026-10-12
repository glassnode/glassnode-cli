package api

import (
	"context"
	"fmt"

	glassnode "github.com/glassnode/glassnode-api-go-client"
)

type APIAddon = glassnode.APIAddon

// CreditsPeriod is the period over which an account's API allowance is
// granted, as /v1/user/api_usage reports it on the add-on.
type CreditsPeriod = glassnode.AllowancePeriod

const (
	CreditsPeriodDaily   = glassnode.AllowanceDaily
	CreditsPeriodMonthly = glassnode.AllowanceMonthly
)

// APIUsageResponse is the SDK's usage report; the CLI adds the summary it
// prints. CreditsUsed is month-to-date and DailyRequestsUsed the current day.
type APIUsageResponse struct {
	glassnode.APIUsage
}

// CreditsSummary is the CLI response to the end user. Every field describes the
// same window: CreditsLimit is the allowance granted per CreditsPeriod, and
// CreditsUsed and CreditsLeft are the usage and remainder within it. When the
// period is not one the CLI knows, CreditsPeriod carries it as reported and
// the usage fields are omitted, since no counter is known to match it.
type CreditsSummary struct {
	CreditsLeft   *int          `json:"creditsLeft,omitempty"`
	CreditsLimit  int           `json:"creditsLimit"`
	CreditsPeriod CreditsPeriod `json:"creditsPeriod"`
	CreditsUsed   *int          `json:"creditsUsed,omitempty"`
}

func (a *APIUsageResponse) Summary() CreditsSummary {
	if allowance, ok := a.Allowance(); ok {
		left, used := allowance.Remaining(), allowance.Used
		return CreditsSummary{CreditsLeft: &left, CreditsLimit: allowance.Limit, CreditsPeriod: allowance.Period, CreditsUsed: &used}
	}
	if len(a.APIAddons) == 0 {
		// No allowance: nothing left of the month-to-date usage.
		left, used := 0, a.CreditsUsed
		return CreditsSummary{CreditsLeft: &left, CreditsPeriod: CreditsPeriodMonthly, CreditsUsed: &used}
	}
	// An unknown or mixed period: report what the API sent rather than
	// measuring it against a counter of another period.
	summary := CreditsSummary{CreditsPeriod: a.APIAddons[0].Period}
	for _, addon := range a.APIAddons {
		summary.CreditsLimit = max(summary.CreditsLimit, addon.Value)
		if addon.Period != summary.CreditsPeriod {
			summary.CreditsPeriod = "mixed"
		}
	}
	return summary
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
	return &APIUsageResponse{APIUsage: *usage}, nil
}
