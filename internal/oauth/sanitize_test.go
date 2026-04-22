package oauth

import (
	"strings"
	"testing"
)

func TestSanitizeForTerm(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty", "", 100, ""},
		{"plain", "hello world", 100, "hello world"},
		{"preserves newline and tab", "a\nb\tc", 100, "a\nb\tc"},
		{"strips ESC", "bad\x1b[31mred", 100, "bad?[31mred"},
		{"strips NUL", "a\x00b", 100, "a?b"},
		{"strips BEL", "a\x07b", 100, "a?b"},
		{"strips DEL", "a\x7fb", 100, "a?b"},
		{"caps length", "abcdefghij", 5, "abcde…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeForTerm(c.in, c.max)
			if got != c.want {
				t.Errorf("sanitizeForTerm(%q,%d) = %q, want %q", c.in, c.max, got, c.want)
			}
		})
	}
}

func TestSanitizeForTerm_NoANSICursorEscapes(t *testing.T) {
	// A representative terminal-injection payload must not survive intact.
	payload := "\x1b[2J\x1b[H\x1b[31mHACKED\x1b[0m"
	got := sanitizeForTerm(payload, 0)
	if strings.Contains(got, "\x1b") {
		t.Errorf("ESC byte must be stripped: got %q", got)
	}
}
