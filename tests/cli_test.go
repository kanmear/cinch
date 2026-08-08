// Package tests exercises the built cinch binary end-to-end (`make test` builds
// bin/cinch before testing).
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellUsage(t *testing.T) {
	out, err := exec.Command("../bin/cinch").CombinedOutput()
	if err == nil {
		t.Fatalf("no-args run: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("no-args run: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "usage:") {
		t.Fatalf("no-args run: usage not printed:\n%s", out)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	out, err := exec.Command("../bin/cinch", "bogus").CombinedOutput()
	if err == nil {
		t.Fatalf("unknown command: want non-zero exit, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("unknown command: want exit code 1, got %v", err)
	}
	if !strings.Contains(string(out), `"bogus" is not a command`) {
		t.Fatalf("unknown command: notice not printed:\n%s", out)
	}
}

func TestCheckNoDocsDir(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check with no .docs dir: want exit 0, got %v\n%s", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("check with no .docs dir: want no output, got:\n%s", out)
	}
}

func TestCheckExitCodesEndToEnd(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir .docs: %v", err)
	}
	content := "see [x](./missing.md) for details\n"
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check with broken link: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "links error") || !strings.Contains(string(out), "missing.md") {
		t.Fatalf("check with broken link: finding not named:\n%s", out)
	}
}

// binPath returns the absolute path to the built cinch binary. Tests that
// set cmd.Dir need this rather than a relative path: exec.Command resolves a
// relative Path against cmd.Dir, not the test process's own working
// directory.
func binPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../bin/cinch")
	if err != nil {
		t.Fatalf("resolve bin path: %v", err)
	}
	return abs
}
