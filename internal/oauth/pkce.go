package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// verifierSeedBytes is the size of the random seed for the PKCE code_verifier. With 64 random
// bytes, the base64url-encoded verifier is 86 characters — well within RFC 7636's 43–128 range
// and giving ~512 bits of entropy.
const verifierSeedBytes = 64

// Pair holds a PKCE code_verifier and its S256 code_challenge (base64url, no padding).
type Pair struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a high-entropy code_verifier and the matching S256 code_challenge per RFC 7636.
func NewPKCE() (Pair, error) {
	b := make([]byte, verifierSeedBytes)
	if _, err := rand.Read(b); err != nil {
		return Pair{}, fmt.Errorf("random verifier: %w", err)
	}

	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return Pair{Verifier: verifier, Challenge: challenge}, nil
}

// RandomState returns a URL-safe random string (256 bits of entropy) for the OAuth state param.
func RandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
