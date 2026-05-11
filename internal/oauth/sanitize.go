package oauth

// sanitizeForTerm strips ASCII control characters (including ANSI/CSI escape bytes) and caps
// the result at max runes. Use on any remote-controlled string before embedding it into an
// error message printed to the user's terminal, to prevent cursor/color/prompt-spoofing
// attacks via `error_description`, token-endpoint error bodies, etc.
func sanitizeForTerm(s string, max int) string {
	if s == "" {
		return ""
	}

	var runes []rune
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n':
			runes = append(runes, r)
		case r < 0x20 || r == 0x7f: // ASCII controls incl. ESC (0x1b) used by ANSI/CSI.
			runes = append(runes, '?')
		default:
			runes = append(runes, r)
		}
	}

	if max > 0 && len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return string(runes)
}
