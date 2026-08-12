package output

import (
	"fmt"
	"os"
)

// This package is cinch's one chokepoint for diagnostic and progress output.
// Every command routes its stderr/stdout messages through these helpers so
// the shape of a message — prefix, scoping, one line per event — is the
// same everywhere, instead of each command hand-building its own "cinch: "
// prefix.

// Fail writes "cinch: <cmd>: <err>\n" to stderr and returns 1, the shared
// shape for a command-scoped failure.
func Fail(cmd string, err error) int {
	return Failf(cmd, "%s", err.Error())
}

// Failf is Fail for a formatted message instead of an error value.
func Failf(cmd, format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s\n", cmd, fmt.Sprintf(format, args...))
	return 1
}

// Skip writes "cinch: <cmd>: <checker>: skip: <msg>\n" to stderr — the
// shared shape for "this didn't run, and that's not a pass" notices.
// Silence must never read as a pass, so a skip always says so explicitly,
// in the same voice as every other diagnostic.
func Skip(cmd, checker, msg string) {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s: skip: %s\n", cmd, checker, msg)
}

// Step writes one stdout progress line for a completed (or deliberately
// no-op) action, verb-first: "wrote cinch.yml", "rendered x.md", "activated
// hooks: ...". Every step announces itself, even when there was nothing to
// do, so a re-run is never silent about what it checked.
func Step(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// UsageErr writes "cinch: <msg>\n" to stderr and returns 2, the shared exit
// code for a malformed invocation.
func UsageErr(msg string) int {
	fmt.Fprintln(os.Stderr, "cinch: "+msg)
	return 2
}
