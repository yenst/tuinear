package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

func OpenBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		command, args = "xdg-open", []string{url}
	}
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
