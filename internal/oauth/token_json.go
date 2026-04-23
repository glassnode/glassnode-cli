package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// maxTokenEndpointBody caps the amount of response body we're willing to buffer from the IdP.
	// Legitimate token responses are well under 16 KiB; larger bodies almost always indicate an
	// upstream error page (HTML, etc.) that we don't want to fully materialize it in memory.
	maxTokenEndpointBody = 64 * 1024

	// maxTermOAuthCodeLen / maxTermOAuthMsgLen bound IdP-controlled strings passed to
	// sanitizeForTerm before printing to the terminal (see sanitize.go).
	maxTermOAuthCodeLen = 64
	maxTermOAuthMsgLen  = 256
)

type tokenJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// tokenEndpointError is used to parse the standard OAuth2 JSON error msg
type tokenEndpointError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func decodeTokenJSON(raw []byte) (string, string, int, error) {
	var tr tokenJSON
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", "", 0, fmt.Errorf("parsing token response: %w", err)
	}

	if tr.AccessToken == "" {
		return "", "", 0, fmt.Errorf("token response missing access_token")
	}

	if tr.TokenType != "" && !strings.EqualFold(tr.TokenType, "bearer") {
		return "", "", 0, fmt.Errorf("unexpected token_type %q, want bearer", sanitizeForTerm(tr.TokenType, maxTermOAuthCodeLen))
	}

	return tr.AccessToken, tr.RefreshToken, tr.ExpiresIn, nil
}

func readTokenResponse(resp *http.Response) (string, string, int, error) {
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenEndpointBody))
	if err != nil {
		return "", "", 0, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Prefer structured OAuth2 error fields over dumping the raw body, which may contain
		// ANSI escapes (terminal injection) or accidentally sensitive data.
		var e tokenEndpointError
		_ = json.Unmarshal(raw, &e)
		if e.Error != "" || e.ErrorDescription != "" {
			return "", "", 0, fmt.Errorf("token endpoint HTTP %d: %s: %s",
				resp.StatusCode,
				sanitizeForTerm(e.Error, maxTermOAuthCodeLen),
				sanitizeForTerm(e.ErrorDescription, maxTermOAuthMsgLen))
		}
		return "", "", 0, fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, sanitizeForTerm(string(raw), maxTermOAuthMsgLen))
	}

	return decodeTokenJSON(raw)
}

func postToken(ctx context.Context, form url.Values) (string, string, int, error) {
	client := defaultHTTPClient
	tokenURL := authDomain + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", 0, fmt.Errorf("token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("token exchange: %w", err)
	}

	return readTokenResponse(resp)
}
