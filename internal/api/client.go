package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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
			transport := httpClient.Transport
			if transport == nil {
				transport = http.DefaultTransport
			}
			auth := &oauthTransport{base: transport, token: c.bearerToken}
			httpClient.Transport = auth
			options = append(options, glassnode.WithTokenSource(auth))
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

// oauthTransport preserves the CLI's refresh-on-401 behavior without teaching
// the SDK about config files or OAuth session persistence.
type oauthTransport struct {
	base  http.RoundTripper
	mu    sync.Mutex
	token string
}

// Token supplies the current session token to the SDK on each attempt.
func (t *oauthTransport) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.token, nil
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	token := t.token
	t.mu.Unlock()
	request := req.Clone(req.Context())
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := t.base.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusUnauthorized {
		return redactOAuthErrorResponse(response, token), err
	}
	t.mu.Lock()
	fresh := t.token
	var refreshErr error
	if fresh == token {
		fresh, refreshErr = oauth.ForceRefreshAccessToken(req.Context())
		if refreshErr == nil && fresh != "" {
			t.token = fresh
		}
	}
	t.mu.Unlock()
	if refreshErr != nil || fresh == "" || fresh == token {
		if refreshErr != nil && !errors.Is(refreshErr, oauth.ErrSessionExpired) {
			_, _ = fmt.Fprintln(stderr, "warning: OAuth refresh after HTTP 401 failed")
		}
		return response, nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	request = req.Clone(req.Context())
	request.Header.Set("Authorization", "Bearer "+fresh)
	response, err = t.base.RoundTrip(request)
	return redactOAuthErrorResponse(response, fresh), err
}

// The SDK knows the original token; the CLI transport also redacts a rotated
// token if an error body echoes it after refresh.
func redactOAuthErrorResponse(response *http.Response, token string) *http.Response {
	if response == nil || response.StatusCode < 300 || token == "" {
		return response
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	if err != nil {
		body = []byte("failed to read OAuth API error response")
	}
	text := strings.ReplaceAll(string(body), token, "[redacted]")
	text = strings.ReplaceAll(text, url.QueryEscape(token), "[redacted]")
	response.Body = io.NopCloser(bytes.NewBufferString(text))
	response.ContentLength = int64(len(text))
	return response
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
