//go:build windows

package cinch

import "fmt"

// reexec is unreachable on windows: maybeInstallLatest skips the
// install/reexec path entirely for runtime.GOOS == "windows". This stub only
// exists so the package still builds for the windows target in the release
// matrix.
func reexec(path string, args, env []string) error {
	return fmt.Errorf("reexec is not supported on windows")
}
