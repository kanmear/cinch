//go:build !windows

package cinch

import "syscall"

// reexec replaces the current process image with path, preserving args and
// env. On success it never returns.
func reexec(path string, args, env []string) error {
	return syscall.Exec(path, args, env)
}
