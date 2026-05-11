package oauth

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
)

const accessExpirySkew = 2 * time.Minute

// ErrSessionExpired indicates that an OAuth session was on disk but is no longer usable
// (e.g. refresh token missing or the IdP rejected it). The caller should prompt the user to
// re-run `gn login`. Distinct from "no session configured at all", which returns ("", nil).
var ErrSessionExpired = errors.New("OAuth session expired; run `gn login` to sign in again")

// EnsureOAuthAccessToken returns a usable OAuth access token, refreshing with the stored refresh
// token when needed.
//
// Return shape:
//   - ("", nil)                  OAuth not configured; caller may fall back to API key.
//   - (token, nil)               valid bearer (possibly freshly refreshed).
//   - ("", ErrSessionExpired)    session was configured but is unusable; user should `gn login`.
//   - ("", other err)            refresh attempted and failed (network / IdP error).
func EnsureOAuthAccessToken(ctx context.Context) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if cfg.OAuthAccessToken == "" && cfg.OAuthRefreshToken == "" {
		return "", nil
	}

	if oauthAccessValid(cfg, accessExpirySkew) {
		return cfg.OAuthAccessToken, nil
	}

	if cfg.OAuthRefreshToken == "" {
		return "", ErrSessionExpired
	}

	return doRefreshAndSave(ctx, cfg.OAuthRefreshToken)
}

// ForceRefreshAccessToken unconditionally exchanges the stored refresh token for a new access
// token (used by the API client when a 401 indicates the current access token was revoked
// server-side before its advertised expiry).
func ForceRefreshAccessToken(ctx context.Context) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if cfg.OAuthRefreshToken == "" {
		return "", ErrSessionExpired
	}

	return doRefreshAndSave(ctx, cfg.OAuthRefreshToken)
}

func doRefreshAndSave(ctx context.Context, refreshToken string) (string, error) {
	access, refresh, exp, err := refreshAccessToken(ctx, refreshToken)
	if err != nil {
		return "", err
	}

	if err := config.SaveOAuthSession(config.OAuthSession{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    exp,
	}); err != nil {
		return "", err
	}

	return access, nil
}

// oauthAccessValid returns true only if the stored access token has a known, not expired
// deadline. If the expiry is unknown (empty), we
// conservatively treat the token as invalid so a refresh is attempted.
func oauthAccessValid(cfg *config.Config, skew time.Duration) bool {
	if cfg.OAuthAccessToken == "" {
		return false
	}

	if cfg.OAuthExpiresAt == "" {
		return false
	}

	t, err := time.Parse(time.RFC3339, cfg.OAuthExpiresAt)
	if err != nil {
		return false
	}

	return time.Now().UTC().Before(t.Add(-skew))
}

func refreshAccessToken(ctx context.Context, oldRefreshToken string) (string, string, time.Time, error) {
	access, newRefresh, expIn, err := postToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {oldRefreshToken},
		"audience":      {audience},
	})
	if err != nil {
		return "", "", time.Time{}, err
	}

	// If the IdP did not rotate the refresh token
	// it returns an empty refresh_token field. Keep the caller's token so we
	// can still refresh on the next expiry.
	if newRefresh == "" {
		newRefresh = oldRefreshToken
	}

	var exp time.Time
	if expIn > 0 {
		exp = time.Now().UTC().Add(time.Duration(expIn) * time.Second)
	}

	return access, newRefresh, exp, nil
}
