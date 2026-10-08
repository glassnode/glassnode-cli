package cmd

import (
	"strings"
	"testing"
)

func TestCredits_DryRun_RedactsKey(t *testing.T) {
	stdout, stderr, err := runCLI(t, []string{"HOME=" + t.TempDir()},
		"user", "credits", "--api-key", "secret-key", "--dry-run",
	)
	if err != nil {
		t.Fatalf("run CLI: %v\nstderr: %s", err, stderr)
	}

	if strings.Contains(stdout, "secret-key") {
		t.Errorf("stdout must not contain API key: %s", stdout)
	}
	// The credits summary needs the account's products to label the period,
	// so the dry run has to show both requests.
	for _, path := range []string{"/v1/user/info", "/v1/user/api_usage"} {
		if !strings.Contains(stdout, path) {
			t.Errorf("stdout should contain path %s: %s", path, stdout)
		}
	}
	if !strings.Contains(stdout, "api_key=***") && !strings.Contains(stdout, "api_key=%2A%2A%2A") {
		t.Errorf("stdout should contain redacted api_key: %s", stdout)
	}
}
