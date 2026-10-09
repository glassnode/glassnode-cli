package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	glassnode "github.com/glassnode/glassnode-api-go-client"
	"github.com/glassnode/glassnode-cli/internal/config"
	"github.com/glassnode/glassnode-cli/internal/oauth"
	"github.com/glassnode/glassnode-cli/internal/version"
)

var stderr io.Writer = os.Stderr

// Client adapts the public SDK to CLI authentication, configuration and output.
// HTTP requests, retries, errors and endpoint decoding are owned by the SDK.
type Client struct {
	baseURL     string
	apiKey      string
	bearerToken string
	httpClient  *http.Client
	init        sync.Once
	sdkClient   *glassnode.Client
	initErr     error
}

func NewClient(apiKey, bearerToken string) *Client {
	baseURL := os.Getenv("GLASSNODE_BASE_URL")
	if baseURL == "" {
		baseURL = glassnode.DefaultBaseURL
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, bearerToken: bearerToken,
		httpClient: &http.Client{Timeout: time.Minute}}
}

func (c *Client) sdk() (*glassnode.Client, error) {
	c.init.Do(func() {
		httpClient := *c.httpClient
		options := []glassnode.Option{
			glassnode.WithBaseURL(c.baseURL),
			glassnode.WithUserAgent("glassnode-cli-" + version.Version),
		}
		key := c.apiKey
		if c.bearerToken != "" {
			options = append(options, glassnode.WithTokenSource(&sessionTokens{token: c.bearerToken}))
			key = ""
		}
		options = append(options, glassnode.WithHTTPClient(&httpClient))
		c.sdkClient, c.initErr = glassnode.NewClient(key, options...)
	})
	return c.sdkClient, c.initErr
}

// AuthHeader names the header that carries the credential. The SDK sends the
// API key in X-Api-Key rather than in the URL, so it stays out of proxy and
// access logs; dry-run output therefore shows the URL without it.
func (c *Client) AuthHeader() string {
	if c.bearerToken != "" {
		return "Authorization: Bearer"
	}
	return "X-Api-Key"
}

// sessionTokens hands the CLI's OAuth session token to the SDK and refreshes
// it when the API rejects it, so a token revoked before its expiry is replaced
// once without teaching the SDK about config files or OAuth persistence.
type sessionTokens struct {
	mu    sync.Mutex
	token string
}

// Token returns the current session token.
func (s *sessionTokens) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token, nil
}

// Refresh exchanges the stored refresh token for a new access token after a
// 401 and keeps it for the following requests. An expired session is reported
// to the user through the SDK's AuthError.
func (s *sessionTokens) Refresh(ctx context.Context) (string, error) {
	fresh, err := oauth.ForceRefreshAccessToken(ctx)
	if err != nil {
		if !errors.Is(err, oauth.ErrSessionExpired) {
			_, _ = fmt.Fprintln(stderr, "warning: OAuth refresh after HTTP 401 failed")
		}
		return "", err
	}
	s.mu.Lock()
	s.token = fresh
	s.mu.Unlock()
	return fresh, nil
}

func queryValues(params map[string]string, repeated map[string][]string) url.Values {
	q := url.Values{}
	for key, value := range params {
		q.Set(key, value)
	}
	for key, values := range repeated {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	return q
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

func (c *Client) DoWithRepeatedParams(ctx context.Context, method, path string, params map[string]string, repeated map[string][]string) ([]byte, error) {
	if method != http.MethodGet {
		return nil, fmt.Errorf("unsupported HTTP method %s", method)
	}
	client, err := c.sdk()
	if err != nil {
		return nil, err
	}
	return client.Raw(ctx, path, queryValues(params, repeated))
}
