package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser launches the user's default browser
func openBrowser(targetURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return start(exec.Command("open", targetURL))
	case "windows":
		return start(exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL))
	default:
		candidates := []string{"xdg-open", "sensible-browser", "x-www-browser"}
		var lastErr error
		for _, bin := range candidates {
			path, err := exec.LookPath(bin)
			if err != nil {
				continue
			}
			cmd := exec.Command(path, targetURL)
			if err := start(cmd); err == nil {
				return nil
			}
			lastErr = err
		}

		if lastErr != nil {
			return lastErr
		}

		return fmt.Errorf("no supported browser launcher found (tried: xdg-open, sensible-browser, x-www-browser)")
	}
}

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", cmd.Path, err)
	}

	return nil
}
