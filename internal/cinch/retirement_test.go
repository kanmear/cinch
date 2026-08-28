package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Seeded defect battery: retirement.go ---
//
// retirement.go had zero test coverage before this file (confirmed by
// repo-wide search) despite the audit doc assuming the HEAD^/HEAD case was
// already covered. These cases close that gap and seed the
// idIsTombstoned substring false-positive named in
// .docs/plans/03-rule-grammar-holes-and-seed-defects.md.

func retirementGitRepo(t *testing.T, root string) func(args ...string) {
	t.Helper()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	return git
}

func TestSeededDefectRetirementNoTombstone(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	git := retirementGitRepo(t, root)

	writeTestFile(t, root, "cinch.yml", "retirement:\n  pattern: \"RETIRED: [A-Z0-9]+-[0-9]+\"\n")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docsRoot, "rules.md", "1. **SIG-001** rule text\n")
	git("add", "-A")
	git("commit", "-q", "-m", "add SIG-001")

	writeTestFile(t, docsRoot, "rules.md", "no rules here\n")
	git("add", "-A")
	git("commit", "-q", "-m", "remove SIG-001 without a tombstone")

	m := loadTestManifest(t, root)
	result := checkRetirement(root, docsRoot, m)

	ok := false
	for _, f := range result.findings {
		if f.check == "retirement" && strings.Contains(f.message, "rule ID present at HEAD^ but absent at HEAD with no recognized tombstone") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a no-tombstone finding for SIG-001", result.findings)
	}
}

func TestSeededDefectRetirementValidTombstone(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	git := retirementGitRepo(t, root)

	writeTestFile(t, root, "cinch.yml", "retirement:\n  pattern: \"RETIRED: [A-Z0-9]+-[0-9]+\"\n")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docsRoot, "rules.md", "1. **SIG-001** rule text\n")
	git("add", "-A")
	git("commit", "-q", "-m", "add SIG-001")

	writeTestFile(t, docsRoot, "rules.md", "RETIRED: SIG-001\n")
	git("add", "-A")
	git("commit", "-q", "-m", "retire SIG-001 with a tombstone")

	m := loadTestManifest(t, root)
	result := checkRetirement(root, docsRoot, m)

	if len(result.findings) != 0 {
		t.Fatalf("findings = %+v, want none (SIG-001 has a recognized tombstone)", result.findings)
	}
}

// TestSeededDefectRetirementWrongIDTombstone reproduces the idIsTombstoned
// substring false-positive named in the audit doc: idIsTombstoned checks
// strings.Contains(matchedTombstoneText, id) rather than requiring an exact
// match, so a tombstone naming an unrelated, longer ID (SIG-020) is
// wrongly accepted as covering a retired ID that happens to be its string
// prefix (SIG-02).
func TestSeededDefectRetirementWrongIDTombstone(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	git := retirementGitRepo(t, root)

	writeTestFile(t, root, "cinch.yml", "retirement:\n  pattern: \"RETIRED: [A-Z0-9]+-[0-9]+\"\n")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docsRoot, "rules.md", "1. **SIG-02** rule text\n")
	git("add", "-A")
	git("commit", "-q", "-m", "add SIG-02")

	// SIG-02 is removed with no real tombstone for it — the only tombstone
	// present names a different, unrelated ID, SIG-020.
	writeTestFile(t, docsRoot, "rules.md", "RETIRED: SIG-020\n")
	git("add", "-A")
	git("commit", "-q", "-m", "remove SIG-02, unrelated SIG-020 tombstone present")

	m := loadTestManifest(t, root)
	result := checkRetirement(root, docsRoot, m)

	// GAP: this is the known idIsTombstoned substring bug — "SIG-02" is a
	// string prefix of "SIG-020", so strings.Contains falsely treats the
	// SIG-020 tombstone as covering SIG-02's retirement and the finding
	// that should fire is suppressed. Not fixed by this item (the audit
	// doc lists it as a seeded case, not a required fix); if
	// idIsTombstoned is later tightened to an exact match, this
	// assertion should flip to expect the finding.
	if len(result.findings) != 0 {
		t.Fatalf("findings = %+v, want none (reproduces the idIsTombstoned substring false-positive)", result.findings)
	}
}

func TestSeededDefectRetirementNoOpSingleCommit(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	git := retirementGitRepo(t, root)

	writeTestFile(t, root, "cinch.yml", "")
	git("add", "-A")
	git("commit", "-q", "-m", "only commit")

	m := loadTestManifest(t, root)
	result := checkRetirement(root, docsRoot, m)

	if result.noOp != "only one commit; no HEAD^ to compare" {
		t.Fatalf("noOp = %q, want the single-commit no-op message", result.noOp)
	}
	if len(result.findings) != 0 {
		t.Fatalf("findings = %+v, want none", result.findings)
	}
}
