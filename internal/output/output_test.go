package output

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestPaint_DisabledReturnsUnchanged(t *testing.T) {
	if got := paint(false, colorRed, "hello"); got != "hello" {
		t.Fatalf("paint(false, ...): want %q unchanged, got %q", "hello", got)
	}
}

func TestPaint_EnabledWrapsInCodeAndReset(t *testing.T) {
	got := paint(true, colorRed, "hello")
	want := string(colorRed) + "hello" + string(colorReset)
	if got != want {
		t.Fatalf("paint(true, ...): want %q, got %q", want, got)
	}
}

// captureStderr redirects os.Stderr for the duration of f. stderrColor is
// computed once at package init against the real os.Stderr, before any test
// runs — under `go test` that's never a terminal, so color stays disabled
// throughout and every expectation below is exact plain text.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

func TestCheckStatus_Ok(t *testing.T) {
	out := captureStderr(t, func() { CheckStatus("links", 0, "") })
	if want := "cinch: check: links: ok\n"; out != want {
		t.Fatalf("CheckStatus ok: want %q, got %q", want, out)
	}
}

func TestCheckStatus_Skip(t *testing.T) {
	out := captureStderr(t, func() {
		CheckStatus("coupling", 0, "working tree matches HEAD, nothing to compare")
	})
	if want := "cinch: check: coupling: skip: working tree matches HEAD, nothing to compare\n"; out != want {
		t.Fatalf("CheckStatus skip: want %q, got %q", want, out)
	}
}

func TestCheckStatus_Findings(t *testing.T) {
	out := captureStderr(t, func() { CheckStatus("links", 2, "") })
	if want := "cinch: check: links: 2 findings\n"; out != want {
		t.Fatalf("CheckStatus findings: want %q, got %q", want, out)
	}

	out = captureStderr(t, func() { CheckStatus("links", 1, "") })
	if want := "cinch: check: links: 1 finding\n"; out != want {
		t.Fatalf("CheckStatus singular finding: want %q, got %q", want, out)
	}
}

func TestLevel_PassesThroughWhenColorDisabled(t *testing.T) {
	for _, level := range []string{"block", "error", "other"} {
		if got := Level(level); got != level {
			t.Fatalf("Level(%q): want unchanged (color disabled in test), got %q", level, got)
		}
	}
}
