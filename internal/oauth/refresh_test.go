package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/glassnode/glassnode-cli/internal/testhelper"
)

func TestOAuthAccessValid_EmptyExpiryTreatedAsInvalid(t *testing.T) {
	// Previously an empty OAuthExpiresAt meant "valid forever" which stranded users once the
	// real token expired. Regression test.
	cfg := &config.Config{OAuthAccessToken: "x", OAuthExpiresAt: ""}
	if oauthAccessValid(cfg, accessExpirySkew) {
		t.Errorf("empty expiry should be treated as invalid (force refresh)")
	}
}

func TestOAuthAccessValid_FutureExpiry(t *testing.T) {
	cfg := &config.Config{
		OAuthAccessToken: "x",
		OAuthExpiresAt:   time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	if !oauthAccessValid(cfg, accessExpirySkew) {
		t.Errorf("future expiry should be valid")
	}
}

func TestOAuthAccessValid_PastExpiry(t *testing.T) {
	cfg := &config.Config{
		OAuthAccessToken: "x",
		OAuthExpiresAt:   time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	}
	if oauthAccessValid(cfg, accessExpirySkew) {
		t.Errorf("past expiry should be invalid")
	}
}

func TestEnsureOAuthAccessToken_NoSession(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		bearer, err := EnsureOAuthAccessToken(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if bearer != "" {
			t.Errorf("bearer=%q, want empty for no-session", bearer)
		}
	})
}

func TestEnsureOAuthAccessToken_AccessOnlyExpiredNoRefresh(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		past := time.Now().UTC().Add(-time.Hour)
		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken: "expired",
			ExpiresAt:   past,
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		bearer, err := EnsureOAuthAccessToken(context.Background())
		if !errors.Is(err, ErrSessionExpired) {
			t.Errorf("want ErrSessionExpired, got bearer=%q err=%v", bearer, err)
		}
	})
}

func TestForceRefreshAccessToken_NoRefreshToken(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		if err := config.SaveOAuthSession(config.OAuthSession{AccessToken: "x"}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		_, err := ForceRefreshAccessToken(context.Background())
		if !errors.Is(err, ErrSessionExpired) {
			t.Errorf("want ErrSessionExpired, got %v", err)
		}
	})
}

func TestForceRefreshAccessToken_SavesNewToken(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"rotated","expires_in":3600,"token_type":"Bearer"}`))
		}))
		defer ts.Close()
		restore := OverrideDefaultsForTesting(ts.URL, "https://api.example", "cid")
		defer restore()

		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "dead",
			RefreshToken: "old-rt",
			ExpiresAt:    time.Now().UTC().Add(-time.Hour),
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}
		got, err := ForceRefreshAccessToken(context.Background())
		if err != nil {
			t.Fatalf("ForceRefreshAccessToken: %v", err)
		}
		if got != "fresh" {
			t.Errorf("got bearer %q, want 'fresh'", got)
		}
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthRefreshToken != "rotated" {
			t.Errorf("refresh token not rotated: %q", cfg.OAuthRefreshToken)
		}
	})
}

func TestRefreshAccessToken_KeepsOldRefreshTokenWhenIdPDoesNotRotate(t *testing.T) {
	// The token endpoint returns a new access token
	// but omits refresh_token. The old refresh token must be preserved so subsequent
	// refreshes still work.
	testhelper.WithTempHome(t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"new-access","expires_in":3600,"token_type":"Bearer"}`))
		}))
		defer ts.Close()
		restore := OverrideDefaultsForTesting(ts.URL, "https://api.example", "cid")
		defer restore()

		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "old-access",
			RefreshToken: "reusable-rt",
			ExpiresAt:    time.Now().UTC().Add(-time.Hour),
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}

		got, err := ForceRefreshAccessToken(context.Background())
		if err != nil {
			t.Fatalf("ForceRefreshAccessToken: %v", err)
		}
		if got != "new-access" {
			t.Errorf("bearer = %q, want new-access", got)
		}

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthRefreshToken != "reusable-rt" {
			t.Errorf("refresh token should be preserved when IdP does not rotate, got %q", cfg.OAuthRefreshToken)
		}
	})
}

func TestReadTokenResponse_SanitizesErrorBody(t *testing.T) {
	// Structured error should be surfaced as parsed fields, with ANSI escapes stripped.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"evil\u001b[31mANSI"}`))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_, _, _, err = readTokenResponse(resp)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "\x1b") {
		t.Errorf("error message leaks ESC byte: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should contain OAuth error code: %q", err.Error())
	}
}

func TestDecodeTokenJSON_EmptyTokenTypeAccepted(t *testing.T) {
	raw := []byte(`{"access_token":"at","expires_in":3600}`)
	a, _, _, err := decodeTokenJSON(raw)
	if err != nil {
		t.Fatalf("decodeTokenJSON: %v", err)
	}
	if a != "at" {
		t.Errorf("access=%q", a)
	}
}

func TestDecodeTokenJSON_RejectsNonBearerType(t *testing.T) {
	raw := []byte(`{"access_token":"at","token_type":"mac"}`)
	_, _, _, err := decodeTokenJSON(raw)
	if err == nil {
		t.Fatal("expected error for non-bearer token_type")
	}
}
