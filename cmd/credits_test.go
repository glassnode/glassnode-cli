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
	if !strings.Contains(stdout, "/v1/user/api_usage") {
		t.Errorf("stdout should contain path: %s", stdout)
	}
	if strings.Contains(stdout, "api_key") {
		t.Errorf("stdout must not contain an api_key parameter: %s", stdout)
	}
	if !strings.Contains(stderr, "X-Api-Key") {
		t.Errorf("stderr should say the key is sent in the X-Api-Key header: %s", stderr)
	}
}
