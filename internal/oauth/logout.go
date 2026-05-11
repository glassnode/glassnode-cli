package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/glassnode/glassnode-cli/internal/config"
)

// RemoteRevokeError is returned by Logout when the IdP revocation request failed but the
// local tokens were cleared successfully.
type RemoteRevokeError struct{ Err error }

func (e *RemoteRevokeError) Error() string { return e.Err.Error() }
func (e *RemoteRevokeError) Unwrap() error { return e.Err }

// Logout revokes the stored refresh token with the IdP (best effort) and then clears all
// OAuth fields from the local config.
//
// Returns *RemoteRevokeError if revocation failed but local tokens were cleared (soft failure).
// Returns a plain error if the local config could not be cleared (hard failure).
func Logout(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	refresh := cfg.OAuthRefreshToken
	access := cfg.OAuthAccessToken
	var remoteErr error

	// Best effort: revoke refresh first (most important; long-lived), then access.
	if refresh != "" {
		if rerr := revokeToken(ctx, refresh); rerr != nil {
			remoteErr = errors.Join(remoteErr, fmt.Errorf("revoke refresh token: %w", rerr))
		}
	}

	if access != "" {
		if aerr := revokeToken(ctx, access); aerr != nil {
			remoteErr = errors.Join(remoteErr, fmt.Errorf("revoke access token: %w", aerr))
		}
	}

	if err := config.ClearOAuthSession(); err != nil {
		return err
	}

	if remoteErr != nil {
		return &RemoteRevokeError{Err: remoteErr}
	}
	return nil
}

func revokeToken(ctx context.Context, token string) error {
	client := defaultHTTPClient

	body := url.Values{
		"client_id": {clientID},
		"token":     {token},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authDomain+"/oauth/revoke",
		strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("revoke HTTP %d", resp.StatusCode)
	}
	return nil
}
