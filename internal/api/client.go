package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/glassnode/glassnode-cli/internal/oauth"
	"github.com/glassnode/glassnode-cli/internal/version"
)

const (
	defaultHTTPTimeout = 1 * time.Minute
)

var (
	baseURL = "https://api.glassnode.com"

	// stderr is where warning messages are written. Overridable in tests.
	stderr io.Writer = os.Stderr
)

type Client struct {
	baseURL     string
	apiKey      string
	bearerToken string
	httpClient  *http.Client
}

func NewClient(apiKey, bearerToken string) *Client {
	if u := os.Getenv("GLASSNODE_BASE_URL"); u != "" {
		baseURL = u
	}

	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		bearerToken: bearerToken,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// ResolveAPIKey returns the first non-empty value from:
// flag value, GLASSNODE_API_KEY env var, config file.
func ResolveAPIKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if env := os.Getenv("GLASSNODE_API_KEY"); env != "" {
		return env
	}

	if val, err := config.Get("api-key"); err == nil && val != "" {
		return val
	}

	return ""
}

// ResolveAuth returns credentials for API calls. OAuth access tokens from gn login take
// precedence (including automatic refresh when a refresh token is stored). Falls back to API
// key (flag, env, then config) when OAuth is not configured or refresh fails.
func ResolveAuth(ctx context.Context, flagAPIKey string) (string, string, error) {
	bearer, oerr := oauth.EnsureOAuthAccessToken(ctx)
	if oerr != nil {
		if key := ResolveAPIKey(flagAPIKey); key != "" {
			_, _ = fmt.Fprintf(stderr, "warning: OAuth unavailable (%v); falling back to API key\n", oerr)
			return key, "", nil
		}
		return "", "", oerr
	}

	if bearer != "" {
		return "", bearer, nil
	}

	return ResolveAPIKey(flagAPIKey), "", nil
}

// RequireAuth returns an error if neither a valid OAuth token nor an API key is available.
func RequireAuth(ctx context.Context, flagAPIKey string) (string, string, error) {
	key, bearer, err := ResolveAuth(ctx, flagAPIKey)
	if err != nil {
		return "", "", err
	}

	if bearer != "" {
		return "", bearer, nil
	}

	if key != "" {
		return key, "", nil
	}

	return "", "", fmt.Errorf("no credentials — run gn login or set an API key (gn config set api-key=… or GLASSNODE_API_KEY)")
}

// RequireAPIKey resolves the API key and returns an error if none is configured.
func RequireAPIKey(flagValue string) (string, error) {
	key := ResolveAPIKey(flagValue)
	if key == "" {
		return "", fmt.Errorf("no API key configured — set one with: gn config set api-key=your-key")
	}
	return key, nil
}

func (c *Client) Do(ctx context.Context, method, path string, params map[string]string) ([]byte, error) {
	return c.DoWithRepeatedParams(ctx, method, path, params, nil)
}

func (c *Client) DoWithRepeatedParams(ctx context.Context, method, path string, params map[string]string, repeatedParams map[string][]string) ([]byte, error) {
	u, err := c.buildURL(path, params, repeatedParams)
	if err != nil {
		return nil, err
	}

	body, status, err := c.sendOnce(ctx, method, u)
	if err != nil {
		return nil, err
	}

	// If we were using an OAuth bearer and the server returned Unauthorized, try exactly one refresh+retry.
	// This handles the case where the access token was revoked before its expiry
	if status == http.StatusUnauthorized && c.bearerToken != "" {
		fresh, rerr := oauth.ForceRefreshAccessToken(ctx)
		if rerr == nil && fresh != "" && fresh != c.bearerToken {
			c.bearerToken = fresh
			body, status, err = c.sendOnce(ctx, method, u)
			if err != nil {
				return nil, err
			}
		} else if rerr != nil && !errors.Is(rerr, oauth.ErrSessionExpired) {
			_, _ = fmt.Fprintf(stderr, "warning: refresh after HTTP %d failed: %v\n", http.StatusUnauthorized, rerr)
		}
	}

	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		if status == http.StatusTooManyRequests {
			return nil, fmt.Errorf("HTTP %d: rate limit exceeded. %s", status, string(body))
		}
		return nil, fmt.Errorf("HTTP %d: %s", status, string(body))
	}
	return body, nil
}

func (c *Client) buildURL(path string, params map[string]string, repeatedParams map[string][]string) (*url.URL, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}
	q := u.Query()
	if c.bearerToken == "" {
		q.Set("api_key", c.apiKey)
	}
	for k, v := range params {
		q.Set(k, v)
	}
	for k, vals := range repeatedParams {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func (c *Client) sendOnce(ctx context.Context, method string, u *url.URL) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "glassnode-cli-"+version.Version)
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}
	return body, resp.StatusCode, nil
}
