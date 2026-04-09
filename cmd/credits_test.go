package cmd

import (
	"strings"
	"testing"
)

func TestCredits_DryRun_RedactsKey(t *testing.T) {
	stdout, stderr, err := runCLI(t, nil,
		"user", "credits", "--api-key", "secret-key", "--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	if strings.Contains(stdout, "secret-key") {
		t.Errorf("stdout must not contain API key: %s", stdout)
	}
	if !strings.Contains(stdout, "/v1/user/api_usage") {
		t.Errorf("stdout should contain path: %s", stdout)
	}
	if !strings.Contains(stdout, "api_key=***") && !strings.Contains(stdout, "api_key=%2A%2A%2A") {
		t.Errorf("stdout should contain redacted api_key: %s", stdout)
	}
}
