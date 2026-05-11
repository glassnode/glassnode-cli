package oauth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
)

const defaultWaitTimeout = 5 * time.Minute

type callbackOutcome struct {
	code string
	err  error
}

// RunLogin starts a local callback server, opens the authorize URL in the browser, exchanges
// the code for tokens, and saves them to config.
func RunLogin(ctx context.Context) error {
	listener, err := bindListener()
	if err != nil {
		return err
	}

	pair, err := NewPKCE()
	if err != nil {
		_ = listener.Close()
		return err
	}

	state, err := RandomState()
	if err != nil {
		_ = listener.Close()
		return err
	}

	authURL, err := buildAuthorizeURL(pair.Challenge, state)
	if err != nil {
		_ = listener.Close()
		return err
	}

	outcome := make(chan callbackOutcome, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", callbackHandler(outcome, state))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	// Single cleanup path covers every exit below. Shutdown (with a short grace period) is
	// preferred over Close so the browser still receives the success/error HTML response body
	// before the socket closes.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	if err := openBrowser(authURL); err != nil {
		return fmt.Errorf("open browser: %w (open this URL manually: %s)", err, authURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, defaultWaitTimeout)
	defer cancel()

	var code string
	select {
	case <-waitCtx.Done():
		return fmt.Errorf("login timed out waiting for browser callback")
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("callback server: %w", err)
		}
		return fmt.Errorf("callback server exited before receiving a response")
	case o := <-outcome:
		if o.err != nil {
			return o.err
		}
		code = o.code
	}

	access, refresh, expIn, err := postToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {pair.Verifier},
	})
	if err != nil {
		return err
	}

	return persistSession(access, refresh, expIn)
}

// buildAuthorizeURL constructs the /authorize URL with all PKCE + OpenID parameters.
func buildAuthorizeURL(challenge, state string) (string, error) {
	u, err := url.Parse(authDomain + "/authorize")
	if err != nil {
		return "", fmt.Errorf("authorize URL: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid profile email offline_access")
	q.Set("audience", audience)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// callbackHandler returns the /callback handler, closing over the buffered outcome channel
// and the expected state.
func callbackHandler(outcome chan<- callbackOutcome, state string) http.HandlerFunc {
	send := func(o callbackOutcome) {
		select {
		case outcome <- o:
		default:
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()

		if qerr := q.Get("error"); qerr != "" {
			// error / error_description are controlled by the IdP redirect (attacker may
			// craft a link). Sanitize before embedding into an error printed to the terminal.
			send(callbackOutcome{err: fmt.Errorf("authorize error: %s: %s",
				sanitizeForTerm(qerr, maxTermOAuthCodeLen),
				sanitizeForTerm(q.Get("error_description"), maxTermOAuthMsgLen))})
			_, _ = fmt.Fprint(w, pageErr)
			return
		}

		got := q.Get("state")
		if subtle.ConstantTimeCompare([]byte(got), []byte(state)) != 1 {
			send(callbackOutcome{err: fmt.Errorf("invalid OAuth state (possible CSRF)")})
			_, _ = fmt.Fprint(w, pageErr)
			return
		}

		code := q.Get("code")
		if code == "" {
			send(callbackOutcome{err: fmt.Errorf("missing authorization code")})
			_, _ = fmt.Fprint(w, pageErr)
			return
		}

		send(callbackOutcome{code: code})
		_, _ = fmt.Fprint(w, pageOK)
	}
}

// persistSession writes token material to config via a single helper shared with refresh.
func persistSession(access, refresh string, expiresIn int) error {
	var exp time.Time
	if expiresIn > 0 {
		exp = time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	}
	return config.SaveOAuthSession(config.OAuthSession{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    exp,
	})
}

func bindListener() (net.Listener, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, fmt.Errorf("redirect URI: %w", err)
	}

	addr := u.Host
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot bind callback listener on %s — another process may be using that port. "+
				"Free up port %s and try again, or check that no other gn login is running",
			addr, u.Port())
	}
	return ln, nil
}
