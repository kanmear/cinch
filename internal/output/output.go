package output

import (
	"fmt"
	"os"
)

// This package is cinch's one chokepoint for diagnostic and progress output.
// Every command routes its stderr/stdout messages through these helpers so
// the shape of a message — prefix, scoping, one line per event — is the
// same everywhere, instead of each command hand-building its own "cinch: "
// prefix. Color is applied the same way: auto-detected per stream, and a
// no-op wherever the destination isn't an actual terminal (piped, redirected,
// or NO_COLOR set), so scripted and hook-driven output stays plain text.

type ansi string

const (
	colorReset   ansi = "\x1b[0m"
	colorRed     ansi = "\x1b[31m"
	colorRedBold ansi = "\x1b[1;31m"
	colorYellow  ansi = "\x1b[33m"
	colorGreen   ansi = "\x1b[32m"
)

// isColorTerminal reports whether f is an actual terminal and the
// NO_COLOR convention (https://no-color.org) hasn't opted out.
func isColorTerminal(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// stdoutColor and stderrColor are computed once per stream, not once
// globally, so redirecting one (`cinch check >out.txt`) doesn't disable
// color on the other.
var (
	stdoutColor = isColorTerminal(os.Stdout)
	stderrColor = isColorTerminal(os.Stderr)
)

// paint wraps s in code when enabled, otherwise returns s unchanged.
func paint(enabled bool, code ansi, s string) string {
	if !enabled {
		return s
	}
	return string(code) + s + string(colorReset)
}

// Fail writes "cinch: <cmd>: error: <err>\n" to stderr and returns 1, the
// shared shape for a command-scoped failure.
func Fail(cmd string, err error) int {
	return Failf(cmd, "%s", err.Error())
}

// Failf is Fail for a formatted message instead of an error value.
func Failf(cmd, format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s %s\n", cmd, paint(stderrColor, colorRed, "error:"), fmt.Sprintf(format, args...))
	return 1
}

// Skip writes "cinch: <cmd>: <checker>: skip: <msg>\n" to stderr — the
// shared shape for "this didn't run, and that's not a pass" notices.
// Silence must never read as a pass, so a skip always says so explicitly,
// in the same voice as every other diagnostic.
func Skip(cmd, checker, msg string) {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s: %s\n", cmd, checker, paint(stderrColor, colorYellow, "skip: "+msg))
}

// Step writes one stdout progress line for a completed (or deliberately
// no-op) action, verb-first: "wrote cinch.yml", "rendered x.md", "activated
// hooks: ...". Every step announces itself, even when there was nothing to
// do, so a re-run is never silent about what it checked.
func Step(format string, args ...any) {
	fmt.Println(paint(stdoutColor, colorGreen, fmt.Sprintf(format, args...)))
}

// UsageErr writes "cinch: <msg> — run 'cinch' for usage.\n" to stderr and
// returns 2, the shared exit code for a malformed invocation.
func UsageErr(msg string) int {
	fmt.Fprintln(os.Stderr, paint(stderrColor, colorRed, "cinch: "+msg+" — run 'cinch' for usage."))
	return 2
}

// ColorizeError wraps s in red when stderr is a color-capable terminal —
// for top-level diagnostics (like the unknown-command notice) that don't
// fit the "cinch: <cmd>: ..." shape Fail/Failf produce.
func ColorizeError(s string) string {
	return paint(stderrColor, colorRed, s)
}

// CheckStatus writes one stderr line reporting a single check's outcome:
// green "ok", red "N findings", or yellow "skip: <reason>" when noOp is
// non-empty. Called once per check, always — not gated on findingCount or
// terminal use — so a run states what it verified, not just what it
// skipped, whether that run is interactive or scripted.
func CheckStatus(check string, findingCount int, noOp string) {
	switch {
	case noOp != "":
		fmt.Fprintf(os.Stderr, "cinch: check: %s: %s\n", check, paint(stderrColor, colorYellow, "skip: "+noOp))
	case findingCount > 0:
		word := "findings"
		if findingCount == 1 {
			word = "finding"
		}
		fmt.Fprintf(os.Stderr, "cinch: check: %s: %s\n", check, paint(stderrColor, colorRed, fmt.Sprintf("%d %s", findingCount, word)))
	default:
		fmt.Fprintf(os.Stderr, "cinch: check: %s: %s\n", check, paint(stderrColor, colorGreen, "ok"))
	}
}

// Plural returns word, suffixed with "s" unless n is 1 — for the "N things:"
// headers the listing commands print. Here rather than in each command so
// `1 workflow` and `2 plans` read the same way whichever command produced
// them.
func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Level colorizes a finding's severity word for stdout — block red+bold,
// error yellow — a no-op (returns level unchanged) when color is disabled.
func Level(level string) string {
	switch level {
	case "block":
		return paint(stdoutColor, colorRedBold, level)
	case "error":
		return paint(stdoutColor, colorYellow, level)
	default:
		return level
	}
}
