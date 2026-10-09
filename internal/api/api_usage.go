package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreditsPeriod is the period over which an account's API credit allowance is granted.
type CreditsPeriod string

const (
	CreditsPeriodDay   CreditsPeriod = "day"
	CreditsPeriodMonth CreditsPeriod = "month"
)

// The closed enum of period values /v1/user/api_usage reports on an addon.
const (
	addonPeriodDaily   = "daily"
	addonPeriodMonthly = "monthly"
)

type APIAddon struct {
	Value   int    `json:"value"`
	Period  string `json:"period"`
	RPM     int    `json:"rpm"`
	Version string `json:"version"`
}

// CreditsPeriod maps the addon's server-side period onto a CreditsPeriod.
// Period is a closed enum of addonPeriodDaily or addonPeriodMonthly, so the
// default is unreachable under the current contract; it keeps the summary
// readable rather than failing if the enum is ever widened, at the cost of
// labelling a new period as monthly.
func (a APIAddon) CreditsPeriod() CreditsPeriod {
	switch a.Period {
	case addonPeriodDaily:
		return CreditsPeriodDay
	case addonPeriodMonthly:
		return CreditsPeriodMonth
	default:
		return CreditsPeriodMonth
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
// when the account has no addons.
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

	return CreditsPeriodMonth
}

// CreditsUsedInPeriod returns the usage that counts against CreditsLimit: the
// current day's requests on a daily allowance, the month-to-date total on a
// monthly one. Both counters reset with their own period, so each only lines up
// with the allowance of the same period.
func (a *APIUsageResponse) CreditsUsedInPeriod() int {
	if a.CreditsPeriod() == CreditsPeriodDay {
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
