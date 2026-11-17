package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type ansi string

const (
	colorReset  ansi = "\x1b[0m"
	colorRed    ansi = "\x1b[31m"
	colorYellow ansi = "\x1b[33m"
	colorGreen  ansi = "\x1b[32m"
	colorDim    ansi = "\x1b[2m"
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

// DefaultWidth is what TerminalWidth reports when the size can't be read.
const DefaultWidth = 100

// IsTerminal reports whether f is a terminal, for commands that lay their
// output out differently for a person than for a pipe.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// TerminalWidth is f's column count, or DefaultWidth when it has none.
func TerminalWidth(f *os.File) int {
	if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
		return width
	}
	return DefaultWidth
}

var (
	stdoutColor = isColorTerminal(os.Stdout)
	stderrColor = isColorTerminal(os.Stderr)

	// stdin is shared by every prompt: a fresh reader per call would let the
	// first one buffer several piped lines and leave the next one empty.
	stdin = bufio.NewReader(os.Stdin)
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

func Warn(cmd, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s %s\n", cmd, paint(stderrColor, colorYellow, "warning:"), fmt.Sprintf(format, args...))
}

// Note is a plain informational line, shaped like Failf and Warn without a label.
func Note(cmd, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cinch: %s: %s\n", cmd, fmt.Sprintf(format, args...))
}

// StatusText colors text green when good and red otherwise, for stderr.
func StatusText(good bool, text string) string {
	if good {
		return paint(stderrColor, colorGreen, text)
	}
	return paint(stderrColor, colorRed, text)
}

// SkipText colors text yellow, the color Skip uses, for stderr.
func SkipText(text string) string {
	return paint(stderrColor, colorYellow, text)
}

func Skip(checker, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", checker, paint(stderrColor, colorYellow, "skip: "+msg))
}

// Step is a plain progress line. Color is reserved for status: green means
// ok, red means failed.
func Step(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// Detail is a Step for secondary information, such as a count of files left
// alone.
func Detail(format string, args ...any) {
	fmt.Println(paint(stdoutColor, colorDim, fmt.Sprintf(format, args...)))
}

// Success is the green closing line of a command that finished its job.
func Success(format string, args ...any) {
	fmt.Println(paint(stdoutColor, colorGreen, fmt.Sprintf(format, args...)))
}

// Dim marks secondary text, for stderr.
func Dim(s string) string {
	return paint(stderrColor, colorDim, s)
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

func ReadLine() string {
	return readLine(stdin)
}

func AskYesNo(prompt string) bool {
	return askYesNo(stdin, prompt)
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func askYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Print(prompt + " [y/N] ")
	switch strings.ToLower(readLine(reader)) {
	case "y", "yes":
		return true
	}
	return false
}

func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func Level(level string) string {
	if level == "error" {
		return paint(stdoutColor, colorRed, level)
	}
	return level
}
