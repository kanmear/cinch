package output

import (
	"bufio"
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
