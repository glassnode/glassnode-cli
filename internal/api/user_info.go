package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CreditsPeriod is the period over which an account's API credit allowance is granted.
type CreditsPeriod string

const (
	CreditsPeriodDay   CreditsPeriod = "day"
	CreditsPeriodMonth CreditsPeriod = "month"
)

// ProductAdvanced is the product whose API credit allowance is metered per day
// instead of per month.
const ProductAdvanced = "advanced"

// UserInfoResponse is the subset of /v1/user/info the CLI relies on. The endpoint
// also returns credentials (apiKey, searchAPIKey) which are deliberately not
// decoded here so they cannot leak into CLI output.
type UserInfoResponse struct {
	UUID     string   `json:"uuid"`
	Email    string   `json:"email"`
	Products []string `json:"products"`
}

// HasProduct reports whether the account holds the named product, case-insensitively.
func (u *UserInfoResponse) HasProduct(name string) bool {
	for _, p := range u.Products {
		if strings.EqualFold(strings.TrimSpace(p), name) {
			return true
		}
	}

	return false
}

// CreditsPeriod returns the period the account's API credit allowance is granted
// over: the advanced product is metered per day, every other product per month.
func (u *UserInfoResponse) CreditsPeriod() CreditsPeriod {
	if u.HasProduct(ProductAdvanced) {
		return CreditsPeriodDay
	}

	return CreditsPeriodMonth
}

// GetUserInfo fetches the account details for the authenticated user
func (c *Client) GetUserInfo(ctx context.Context) (*UserInfoResponse, error) {
	body, err := c.Do(ctx, "GET", "/v1/user/info", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching user info: %w", err)
	}

	var out UserInfoResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding user info response: %w", err)
	}

	return &out, nil
}
