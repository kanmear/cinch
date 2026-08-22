package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckCommit(t *testing.T) {
	t.Run("no message file", func(t *testing.T) {
		r := checkCommit(t.TempDir(), "")
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("pattern not configured", func(t *testing.T) {
		r := checkCommit(t.TempDir(), "msg.txt")
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "cinch.yml", "commit:\n  pattern: '['\n")
		r := checkCommit(dir, "msg.txt")
		if len(r.findings) != 1 || r.findings[0].check != "commit" {
			t.Fatalf("result = %+v, want commit finding", r)
		}
	})

	t.Run("unreadable message file", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "cinch.yml", "commit:\n  pattern: '^feat'\n")
		r := checkCommit(dir, filepath.Join(dir, "missing.txt"))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("matches subject only", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, dir, "msg.txt", "feat: add things\n\nbody line\n")
		r := checkCommit(dir, msg)
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("does not match", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, dir, "msg.txt", "fix: nope\n")
		r := checkCommit(dir, msg)
		if len(r.findings) != 1 {
			t.Fatalf("result = %+v, want one finding", r)
		}
		f := r.findings[0]
		if f.check != "commit" || f.level != "error" || f.file != msg || f.line != 1 {
			t.Fatalf("finding = %+v", f)
		}
	})
}
