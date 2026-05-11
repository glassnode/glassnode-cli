package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/glassnode/glassnode-cli/internal/testhelper"
)

func TestLogout_RevokesAndClears(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		var revokeCalls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/revoke" {
				t.Errorf("unexpected path %s", r.URL.Path)
			}
			atomic.AddInt32(&revokeCalls, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		restore := OverrideDefaultsForTesting(ts.URL, "https://api.example", "cid")
		defer restore()

		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresAt:    time.Now().UTC().Add(time.Hour),
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}

		err := Logout(context.Background())
		if err != nil {
			t.Fatalf("Logout: %v", err)
		}
		// Both access and refresh should be revoked.
		if got := atomic.LoadInt32(&revokeCalls); got != 2 {
			t.Errorf("revoke calls = %d, want 2", got)
		}

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthAccessToken != "" || cfg.OAuthRefreshToken != "" || cfg.OAuthExpiresAt != "" {
			t.Errorf("local OAuth fields not cleared: %+v", cfg)
		}
	})
}

func TestLogout_ClearsEvenWhenRemoteFails(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		restore := OverrideDefaultsForTesting(ts.URL, "https://api.example", "cid")
		defer restore()

		if err := config.SaveOAuthSession(config.OAuthSession{
			AccessToken:  "at",
			RefreshToken: "rt",
		}); err != nil {
			t.Fatalf("SaveOAuthSession: %v", err)
		}

		err := Logout(context.Background())
		var remoteErr *RemoteRevokeError
		if !errors.As(err, &remoteErr) {
			t.Fatalf("expected *RemoteRevokeError, got %v", err)
		}
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.OAuthRefreshToken != "" {
			t.Errorf("local tokens should still be cleared on remote failure: %+v", cfg)
		}
	})
}

func TestLogout_NoSession(t *testing.T) {
	testhelper.WithTempHome(t, func() {
		err := Logout(context.Background())
		if err != nil {
			t.Fatalf("Logout: %v", err)
		}
	})
}
