package oauth2

import (
	"os/exec"
	"runtime"
)

// browserCommand returns the executable name and extra args for opening a URL
// on the given OS. Extracted for testability.
func browserCommand(goos string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		return "xdg-open", nil
	}
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(u string) error {
	name, args := browserCommand(runtime.GOOS)
	return exec.Command(name, append(args, u)...).Start() // #nosec G204 -- u is an OAuth authorization URL constructed from trusted config, not user input
}
