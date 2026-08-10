package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=cinch-test",
		"GIT_AUTHOR_EMAIL=cinch-test@example.com",
		"GIT_COMMITTER_NAME=cinch-test",
		"GIT_COMMITTER_EMAIL=cinch-test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	return dir
}

func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// seedCoupledRule commits a doc with two rules (CIN-001, CIN-002) each with
// a marker in its own test file, so callers can mutate one and assert the
// other stays unaffected.
func seedCoupledRule(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text.\n"+
			"2. **CIN-002** the second rule's text.\n")
	writeFile(t, filepath.Join(dir, "foo_test.go"), marker("CIN-001")+"\nfunc TestFoo(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "bar_test.go"), marker("CIN-002")+"\nfunc TestBar(t *testing.T) {}\n")
	gitCommitAll(t, dir, "seed")
}

func TestCoupling_TextChangedMarkerUntouchedFires(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	// Mutate the *second* line of CIN-001's item text — proves item-scoping,
	// not just a diff of the opening line.
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text, CHANGED.\n"+
			"2. **CIN-002** the second rule's text.\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if result.NoOp != "" {
		t.Fatalf("want a real comparison, got no-op: %s", result.NoOp)
	}
	if len(result.Findings) != 1 || !strings.HasPrefix(result.Findings[0].Message, "CIN-001") {
		t.Fatalf("want 1 finding for CIN-001, got %+v", result.Findings)
	}
}

func TestCoupling_TextAndMarkerBothChangedIsClean(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text, CHANGED.\n"+
			"2. **CIN-002** the second rule's text.\n")
	writeFile(t, filepath.Join(dir, "foo_test.go"), marker("CIN-001")+"\nfunc TestFoo(t *testing.T) { /* updated */ }\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
}

func TestCoupling_CleanTreeAnnouncesNoOp(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message for a clean tree, got none")
	}
}

func TestCoupling_UntrackedMarkerFileCountsAsChanged(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	// Change CIN-001's text and move its marker into a brand-new file that
	// is never staged. `git diff HEAD` alone doesn't see untracked files —
	// only `git ls-files --others` does — so without unioning the two, this
	// false-positives a block finding even though the marker did move.
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text, CHANGED.\n"+
			"2. **CIN-002** the second rule's text.\n")
	if err := os.Remove(filepath.Join(dir, "foo_test.go")); err != nil {
		t.Fatalf("remove foo_test.go: %v", err)
	}
	writeFile(t, filepath.Join(dir, "foo_new_test.go"), marker("CIN-001")+"\nfunc TestFoo(t *testing.T) { /* updated */ }\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings — untracked marker file should count as changed, got %+v", result.Findings)
	}
}

func TestCoupling_DocsRootOutsideRepoAnnouncesNoOp(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	// A real, uncommitted change so the check doesn't take the unrelated
	// "working tree matches HEAD" no-op path — the only thing under test
	// here is the paths.docs-escapes-the-repo guard.
	writeFile(t, filepath.Join(dir, "foo_test.go"), marker("CIN-001")+"\nfunc TestFoo(t *testing.T) { /* updated */ }\n")

	outside := t.TempDir()

	result := checkCoupling(outside, dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if !strings.Contains(result.NoOp, "outside the repository") {
		t.Fatalf("want a no-op naming paths.docs as outside the repository, got: %q", result.NoOp)
	}
}

func TestCoupling_NotAGitRepoAnnouncesNoOp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"), "1. **CIN-001** text.\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message for a non-git directory, got none")
	}
}

func TestCoupling_RuleRewordEscapeSuppresses(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text, CHANGED.\n"+
			"2. **CIN-002** the second rule's text.\n")

	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, msgFile, "reword CIN-001's phrasing\n\nrule-reword: CIN-001\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, msgFile)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings (suppressed), got %+v", result.Findings)
	}
	if len(result.Suppressed) != 1 {
		t.Fatalf("want the suppression logged, got %+v", result.Suppressed)
	}
}

func TestCoupling_UnrelatedRuleUnaffected(t *testing.T) {
	dir := gitInitRepo(t)
	seedCoupledRule(t, dir)

	// Only CIN-002's text and marker change; CIN-001 must stay untouched.
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, opening line.\n"+
			"   Second line of the first rule's text.\n"+
			"2. **CIN-002** the second rule's text, CHANGED.\n")
	writeFile(t, filepath.Join(dir, "bar_test.go"), marker("CIN-002")+"\nfunc TestBar(t *testing.T) { /* updated */ }\n")

	result := checkCoupling(filepath.Join(dir, ".docs"), dir, "")

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
}
