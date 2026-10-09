package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CreditsPeriod is the period over which an account's API credit allowance is
// granted. The values are the ones /v1/user/api_usage reports on the addon, so
// they reach the CLI output unchanged rather than being renamed on the way.
type CreditsPeriod string

const (
	CreditsPeriodDaily   CreditsPeriod = "daily"
	CreditsPeriodMonthly CreditsPeriod = "monthly"
)

type APIAddon struct {
	Value   int    `json:"value"`
	Period  string `json:"period"`
	RPM     int    `json:"rpm"`
	Version string `json:"version"`
}

// CreditsPeriod returns the addon's period as a CreditsPeriod. Period is a
// closed enum of CreditsPeriodDaily or CreditsPeriodMonthly, matched
// case-insensitively so a change in the server's casing or padding cannot
// silently route a daily allowance onto the month-to-date counter. The default
// keeps the summary readable rather than failing if the enum is ever widened,
// at the cost of labelling a new period as monthly.
func (a APIAddon) CreditsPeriod() CreditsPeriod {
	switch strings.ToLower(strings.TrimSpace(a.Period)) {
	case string(CreditsPeriodDaily):
		return CreditsPeriodDaily
	case string(CreditsPeriodMonthly):
		return CreditsPeriodMonthly
	default:
		return CreditsPeriodMonthly
	}
}

type APIUsageResponse struct {
	CustomerID int `json:"customerId"`
	// CreditsUsed is month-to-date and DailyRequestsUsed is the current day,
	// so each pairs with the addon period of the same name.
	CreditsUsed       int        `json:"creditsUsed"`
	DailyRequestsUsed int        `json:"dailyRequestsUsed"`
	APIAddons         []APIAddon `json:"apiAddons"`
}

// primaryAddon returns the addon carrying the largest credit allowance, or nil
// when the account has no addons. An account cannot hold both a monthly and a
// daily API addon, so this never has to choose between periods: the max only
// guards against ordering assumptions, it is not a cross-period comparison.
func (a *APIUsageResponse) primaryAddon() *APIAddon {
	var primary *APIAddon
	for i := range a.APIAddons {
		if primary == nil || a.APIAddons[i].Value > primary.Value {
			primary = &a.APIAddons[i]
		}
	}

	return primary
}

// CreditsLimit returns the largest addon credit value, or 0 when there are no addons
func (a *APIUsageResponse) CreditsLimit() int {
	if primary := a.primaryAddon(); primary != nil {
		return primary.Value
	}

	return 0
}

// CreditsPeriod returns the period the allowance reported by CreditsLimit is
// granted over, taken from the addon that allowance comes from.
func (a *APIUsageResponse) CreditsPeriod() CreditsPeriod {
	if primary := a.primaryAddon(); primary != nil {
		return primary.CreditsPeriod()
	}

	return CreditsPeriodMonthly
}

// CreditsUsedInPeriod returns the usage that counts against CreditsLimit: the
// current day's requests on a daily allowance, the month-to-date total on a
// monthly one. Both counters reset with their own period, so each only lines up
// with the allowance of the same period.
func (a *APIUsageResponse) CreditsUsedInPeriod() int {
	if a.CreditsPeriod() == CreditsPeriodDaily {
		return a.DailyRequestsUsed
	}

	return a.CreditsUsed
}

// CreditsSummary is the CLI response to the end user. Every field describes the
// same window: CreditsLimit is the allowance granted per CreditsPeriod, and
// CreditsUsed and CreditsLeft are the usage and remainder within it.
type CreditsSummary struct {
	CreditsLeft   int           `json:"creditsLeft"`
	CreditsLimit  int           `json:"creditsLimit"`
	CreditsPeriod CreditsPeriod `json:"creditsPeriod"`
	CreditsUsed   int           `json:"creditsUsed"`
}

func (a *APIUsageResponse) Summary() CreditsSummary {
	limit := a.CreditsLimit()
	used := a.CreditsUsedInPeriod()
	left := limit - used
	if left < 0 {
		left = 0
	}

	return CreditsSummary{
		CreditsUsed:   used,
		CreditsLimit:  limit,
		CreditsPeriod: a.CreditsPeriod(),
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
