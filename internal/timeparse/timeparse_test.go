package timeparse

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Run("unix_timestamp", func(t *testing.T) {
		got, err := Parse("1704067200")
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got != 1704067200 {
			t.Errorf("got %d, want 1704067200", got)
		}
	})

	t.Run("iso_date", func(t *testing.T) {
		got, err := Parse("2024-01-01")
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got != 1704067200 {
			t.Errorf("got %d, want 1704067200", got)
		}
	})

	t.Run("rfc3339", func(t *testing.T) {
		got, err := Parse("2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got != 1704067200 {
			t.Errorf("got %d, want 1704067200", got)
		}
	})

	t.Run("relative_30d", func(t *testing.T) {
		before := time.Now()
		got, err := Parse("30d")
		after := time.Now()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		wantMin := before.Add(-30*24*time.Hour - 5*time.Second).Unix()
		wantMax := after.Add(-30*24*time.Hour + 5*time.Second).Unix()
		if got < wantMin || got > wantMax {
			t.Errorf("got %d, want between %d and %d (now minus 30 days ±5s)", got, wantMin, wantMax)
		}
	})

	t.Run("relative_1h", func(t *testing.T) {
		before := time.Now()
		got, err := Parse("1h")
		after := time.Now()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		wantMin := before.Add(-time.Hour - 5*time.Second).Unix()
		wantMax := after.Add(-time.Hour + 5*time.Second).Unix()
		if got < wantMin || got > wantMax {
			t.Errorf("got %d, want between %d and %d (now minus 1 hour ±5s)", got, wantMin, wantMax)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		_, err := Parse("")
		if err == nil {
			t.Error("expected error for empty string")
		}
	})

	t.Run("invalid_abc", func(t *testing.T) {
		_, err := Parse("abc")
		if err == nil {
			t.Error("expected error for abc")
		}
	})

	t.Run("invalid_30x", func(t *testing.T) {
		_, err := Parse("30x")
		if err == nil {
			t.Error("expected error for 30x")
		}
	})
}
