package output

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type ansi string

const (
	colorReset   ansi = "\x1b[0m"
	colorRed     ansi = "\x1b[31m"
	colorRedBold ansi = "\x1b[1;31m"
	colorYellow  ansi = "\x1b[33m"
	colorGreen   ansi = "\x1b[32m"
)

func isColorTerminal(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func IsInteractiveStdin(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

var (
	stdoutColor = isColorTerminal(os.Stdout)
	stderrColor = isColorTerminal(os.Stderr)
)

func paint(enabled bool, code ansi, s string) string {
	if !enabled {
		return s
	}
	return string(code) + s + string(colorReset)
}

func Fail(cmd string, err error) int {
	return Failf(cmd, "%s", err.Error())
}

func Failf(cmd, format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s %s\n", cmd, paint(stderrColor, colorRed, "error:"), fmt.Sprintf(format, args...))
	return 1
}

func Skip(checker, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", checker, paint(stderrColor, colorYellow, "skip: "+msg))
}

func Step(format string, args ...any) {
	fmt.Println(paint(stdoutColor, colorGreen, fmt.Sprintf(format, args...)))
}

func UsageError(msg string) int {
	fmt.Fprintln(os.Stderr, paint(stderrColor, colorRed, "cinch: "+msg+" — run 'cinch help' for usage."))
	return 2
}

func CheckStatus(check string, findingCount int, noOp string, detail string) {
	switch {
	case noOp != "":
		fmt.Fprintf(os.Stderr, "%s: %s\n", check, paint(stderrColor, colorYellow, "skip: "+noOp))
	case findingCount > 0:
		word := "findings"
		if findingCount == 1 {
			word = "finding"
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", check, paint(stderrColor, colorRed, fmt.Sprintf("%d %s", findingCount, word)))
	default:
		ok := "ok"
		if detail != "" {
			ok += " " + detail
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", check, paint(stderrColor, colorGreen, ok))
	}
}

func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

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
