package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, root, msg string) {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", msg)
}

func docRule(id, text string) string {
	return "# Rules\n\n1. **" + id + "** " + text + "\n"
}

func newCouplingRepo(t *testing.T) (root, docs string) {
	t.Helper()
	root = t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test User")
	docs = filepath.Join(root, ".docs")
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "original text"))
	writeTestFile(t, root, "tests/marker_test.go", "package tests\n// cinch:rule R-001\n")
	commitAll(t, root, "baseline")
	return root, docs
}

func TestCouplingStagedIgnoresUnstagedDocEdit(t *testing.T) {
	root, docs := newCouplingRepo(t)
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "edited but not staged"))
	writeTestFile(t, root, "README.md", "unrelated staged file\n")
	git(t, root, "add", "README.md")

	if res := checkCoupling(docs, root, "", true); len(res.Findings) != 0 {
		t.Fatalf("staged mode blocked on unstaged doc edit: %+v", res.Findings)
	}
	if res := checkCoupling(docs, root, "", false); len(res.Findings) != 1 {
		t.Fatalf("worktree mode should still report the unstaged edit: %+v", res.Findings)
	}
}

func TestCouplingStagedBlocksDocEditWithoutMarker(t *testing.T) {
	root, docs := newCouplingRepo(t)
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "staged new text"))
	git(t, root, "add", ".docs/rules.md")

	res := checkCoupling(docs, root, "", true)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d (%+v)", len(res.Findings), res.Findings)
	}
}

func TestCouplingStagedPassesWithStagedMarker(t *testing.T) {
	root, docs := newCouplingRepo(t)
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "staged new text"))
	writeTestFile(t, root, "tests/marker_test.go", "package tests\n// cinch:rule R-001 changed\n")
	git(t, root, "add", "-A")

	if res := checkCoupling(docs, root, "", true); len(res.Findings) != 0 {
		t.Fatalf("expected no findings, got %+v", res.Findings)
	}
}

func TestCouplingStagedBlocksWhenMarkerChangeUnstaged(t *testing.T) {
	root, docs := newCouplingRepo(t)
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "staged new text"))
	git(t, root, "add", ".docs/rules.md")
	writeTestFile(t, root, "tests/marker_test.go", "package tests\n// cinch:rule R-001 changed\n")

	if res := checkCoupling(docs, root, "", true); len(res.Findings) != 1 {
		t.Fatalf("staged mode should block: %+v", res.Findings)
	}
	if res := checkCoupling(docs, root, "", false); len(res.Findings) != 0 {
		t.Fatalf("worktree mode masks the unstaged marker (old bug): %+v", res.Findings)
	}
}

func TestCouplingStagedRuleReword(t *testing.T) {
	root, docs := newCouplingRepo(t)
	writeTestFile(t, root, ".docs/rules.md", docRule("R-001", "staged new text"))
	git(t, root, "add", ".docs/rules.md")

	msgFile := filepath.Join(root, "msg.txt")
	writeTestFile(t, root, "msg.txt", "rule-reword: R-001\n")

	res := checkCoupling(docs, root, msgFile, true)
	if len(res.Findings) != 0 {
		t.Fatalf("rule-reword should suppress: %+v", res.Findings)
	}
	if len(res.Suppressed) != 1 {
		t.Fatalf("expected 1 suppressed, got %+v", res.Suppressed)
	}
}

func TestCouplingStagedNoOpWhenNothingStaged(t *testing.T) {
	root, docs := newCouplingRepo(t)
	res := checkCoupling(docs, root, "", true)
	if !strings.Contains(res.NoOp, "nothing staged") {
		t.Fatalf("expected nothing-staged NoOp, got %q", res.NoOp)
	}
}
