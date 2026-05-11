package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewPKCE(t *testing.T) {
	pair, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE: %v", err)
	}
	if len(pair.Verifier) < 43 || len(pair.Verifier) > 128 {
		t.Errorf("verifier length %d, want within RFC 7636 bounds [43,128]", len(pair.Verifier))
	}
	if pair.Challenge == "" {
		t.Error("empty challenge")
	}
	sum := sha256.Sum256([]byte(pair.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if pair.Challenge != want {
		t.Errorf("challenge %q, want %q", pair.Challenge, want)
	}
}

func TestRandomState(t *testing.T) {
	s, err := RandomState()
	if err != nil {
		t.Fatalf("RandomState: %v", err)
	}
	// 32 random bytes -> 43-char base64url-nopad string.
	if len(s) < 43 {
		t.Errorf("state too short: len=%d %q", len(s), s)
	}

	s2, err := RandomState()
	if err != nil {
		t.Fatalf("RandomState: %v", err)
	}
	if s == s2 {
		t.Errorf("RandomState returned identical values across calls: %q", s)
	}
}
