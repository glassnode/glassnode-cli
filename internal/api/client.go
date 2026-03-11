package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/glassnode/glassnode-cli/internal/config"
)

const (
	defaultHTTPTimeout = 1 * time.Minute
)

var (
	baseURL = "https://api.glassnode.com"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	if u := os.Getenv("GLASSNODE_BASE_URL"); u != "" {
		baseURL = u
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
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
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	q := u.Query()
	q.Set("api_key", c.apiKey)
	for k, v := range params {
		q.Set(k, v)
	}
	for k, vals := range repeatedParams {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("HTTP 429: rate limit exceeded. %s", string(body))
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
