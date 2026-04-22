package oauth

import (
	"net/http"
	"time"
)

var (
	// NOTE: these are `var` (not `const`) so we can swap the values in tests.
	authDomain = "https://auth.glassnode.com"
	audience   = "https://api.glassnode.com"
	// public OAuth client ID
	clientID = "824OtHVQX0IY1gwfednFYcrMkfrp2zCq"
)

const redirectURI = "http://127.0.0.1:9999/callback"

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Because it mutates shared state without sync, tests that call this
// function must NOT call t.Parallel(), doing so could introduce data races with other tests.
func OverrideDefaultsForTesting(newAuthDomain, newAudience, newClientID string) func() {
	origAuthDomain, origAudience, origClientID := authDomain, audience, clientID
	authDomain = newAuthDomain
	audience = newAudience
	clientID = newClientID
	return func() {
		authDomain = origAuthDomain
		audience = origAudience
		clientID = origClientID
	}
}
