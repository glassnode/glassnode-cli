package oauth

import (
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestBindListener_BusyPortReturnsUserFriendlyError(t *testing.T) {
	// Occupy a port so the listener fails; the error should tell the user what to do.
	busy, err := net.Listen("tcp", "127.0.0.1:9999")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		_ = busy.Close()
	}()

	_, err = bindListener()
	if err == nil {
		t.Fatal("expected error when port is busy")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Errorf("error should mention port, got: %v", err)
	}
}

func TestBindListener_Success(t *testing.T) {
	ln, err := bindListener()
	if err != nil {
		t.Fatalf("bindListener() failed (port %s may already be in use): %v", "9999", err)
	}
	defer func() {
		_ = ln.Close()
	}()

	if !strings.Contains(redirectURI, "127.0.0.1") {
		t.Errorf("redirect %q should contain loopback host", redirectURI)
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	raw, err := buildAuthorizeURL("challenge123", "state456")
	if err != nil {
		t.Fatalf("buildAuthorizeURL: %v", err)
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	q := u.Query()
	cases := []struct{ param, want string }{
		{"response_type", "code"},
		{"client_id", clientID},
		{"redirect_uri", redirectURI},
		{"scope", "openid profile email offline_access"},
		{"audience", audience},
		{"code_challenge", "challenge123"},
		{"code_challenge_method", "S256"},
		{"state", "state456"},
	}
	for _, c := range cases {
		if got := q.Get(c.param); got != c.want {
			t.Errorf("param %q = %q, want %q", c.param, got, c.want)
		}
	}

	if !strings.HasSuffix(u.Path, "/authorize") {
		t.Errorf("path %q should end with /authorize", u.Path)
	}
}
