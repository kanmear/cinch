package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHookChecksSeparate pins the pre-commit/commit-msg split: the static
// suite catches doc issues, while commit-msg checks only the subject pattern
// and is unaffected by broken docs.
func TestHookChecksSeparate(t *testing.T) {
	root := t.TempDir()

	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "[broken](docs.md)\n")
	writeTestFile(t, root, "cinch.yml", "commit:\n  pattern: '^docs: .+'\n")
	good := writeTestFile(t, root, "msg-good.txt", "docs: remove completed plan\n")
	bad := writeTestFile(t, root, "msg-bad.txt", "oops: not matching\n")

	if got := preCommitChecks(root); got != 1 {
		t.Fatalf("preCommitChecks = %d, want 1 (broken doc link)", got)
	}
	if got := commitMsgChecks(root, good); got != 0 {
		t.Fatalf("commitMsgChecks(matching) = %d, want 0 (broken docs must not leak in)", got)
	}
	if got := commitMsgChecks(root, bad); got != 1 {
		t.Fatalf("commitMsgChecks(non-matching) = %d, want 1", got)
	}
}
