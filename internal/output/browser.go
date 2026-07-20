package output

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser launches url in the user's default browser, dispatching on GOOS.
// Returns an error when no opener is found or it fails to start — callers fall
// back to printing the URL. Uses Start (not Run) so we don't block on the
// browser. Mirrors the external-process launch in pager.go.
func OpenBrowser(url string) error {
	var argv []string
	switch runtime.GOOS {
	case "darwin":
		argv = []string{"open", url}
	case "windows":
		argv = []string{"rundll32", "url.dll,FileProtocolHandler", url}
	default:
		argv = []string{"xdg-open", url}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("no browser opener (%s) found: %w", argv[0], err)
	}
	return exec.Command(path, argv[1:]...).Start()
}
