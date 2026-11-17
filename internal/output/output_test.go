package output

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"
)

func TestAskYesNo(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes", "yes\n", true},
		{"upper Y", "Y\n", true},
		{"lower y", "y\n", true},
		{"mixed-case yes", "YeS\n", true},
		{"padded", "  y  \n", true},
		{"y without newline", "y", true},
		{"empty line", "\n", false},
		{"no input", "", false},
		{"n", "n\n", false},
		{"nope", "nope\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.input))
			if got := askYesNo(reader, "go?"); got != tc.want {
				t.Fatalf("askYesNo(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestReadLineSharesOneReader(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("a\nb\n"))

	if got := readLine(reader); got != "a" {
		t.Fatalf("first readLine = %q, want %q", got, "a")
	}
	if got := readLine(reader); got != "b" {
		t.Fatalf("second readLine = %q, want %q", got, "b")
	}
	if got := readLine(reader); got != "" {
		t.Fatalf("readLine at EOF = %q, want empty", got)
	}
}

func TestAskYesNoThenReadLineOnOneReader(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("yes\nvalue\n"))

	if !askYesNo(reader, "go?") {
		t.Fatal("askYesNo = false, want true")
	}
	if got := readLine(reader); got != "value" {
		t.Fatalf("readLine after askYesNo = %q, want %q", got, "value")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = original
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func withColor(t *testing.T) {
	t.Helper()
	oldOut, oldErr := stdoutColor, stderrColor
	stdoutColor, stderrColor = true, true
	t.Cleanup(func() { stdoutColor, stderrColor = oldOut, oldErr })
}

// Green means ok and nothing else: a progress line carries no status.
func TestStepIsPlainAndSuccessIsGreen(t *testing.T) {
	withColor(t)

	if got := captureStdout(t, func() { Step("wrote %s", "cinch.yml") }); got != "wrote cinch.yml\n" {
		t.Fatalf("Step = %q, want plain text", got)
	}
	if got := captureStdout(t, func() { Detail("%d files up to date", 3) }); got != "\x1b[2m3 files up to date\x1b[0m\n" {
		t.Fatalf("Detail = %q, want dim text", got)
	}
	if got := captureStdout(t, func() { Success("done") }); got != "\x1b[32mdone\x1b[0m\n" {
		t.Fatalf("Success = %q, want green text", got)
	}
}

func TestLevelColorsOnlyError(t *testing.T) {
	withColor(t)

	if got := Level("error"); got != "\x1b[31merror\x1b[0m" {
		t.Fatalf("Level(error) = %q, want red", got)
	}
	if got := Level("warning"); got != "warning" {
		t.Fatalf("Level(warning) = %q, want plain", got)
	}
}

// A skipped hook step is neither ok nor failed: it takes Skip's yellow.
func TestSkipTextIsYellowNotGreenOrRed(t *testing.T) {
	withColor(t)

	if got := SkipText("skipped"); got != "\x1b[33mskipped\x1b[0m" {
		t.Fatalf("SkipText = %q, want yellow", got)
	}
	if SkipText("skipped") == StatusText(true, "skipped") || SkipText("skipped") == StatusText(false, "skipped") {
		t.Fatal("SkipText matches StatusText, want a third color")
	}
	if got := captureStderr(t, func() { Skip("lint", "no files") }); !strings.Contains(got, "\x1b[33m") {
		t.Fatalf("Skip = %q, want the same yellow", got)
	}
}

func TestDimWithoutColorIsPlain(t *testing.T) {
	old := stderrColor
	stderrColor = false
	t.Cleanup(func() { stderrColor = old })

	if got := Dim("lint"); got != "lint" {
		t.Fatalf("Dim = %q, want plain when stderr is not a color terminal", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = original
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
