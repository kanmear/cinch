package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cinch:rule CINCH-011
// catch-matrix: rules | not a defect | tracked file deleted from disk but not staged (CINCH-011)
func TestRulesCheckSkipsTrackedFileDeletedFromDisk(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, "internal"), "gone.go", "package auth\n")
	git("add", "-A")
	git("commit", "-q", "-m", "add gone.go")
	if err := os.Remove(filepath.Join(root, "internal", "gone.go")); err != nil {
		t.Fatal(err)
	}

	var got int
	out := captureStdout(t, func() { got = runChecks(workingTreeRoots(root), "", false, "rules") })
	if got != 0 {
		t.Fatalf("runChecks(rules) = %d, want 0 (a tracked file missing from disk is not a scan error)\n%s", got, out)
	}
}

// cinch:rule CINCH-011
func TestRulesCheckSkipsTrackedSymlinkToDirectory(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	if err := os.Symlink("internal", filepath.Join(root, "link-to-internal")); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "add symlink")

	report := checkRules(workingTreeRoots(root), filepath.Join(root, ".docs"), markerScanOptions{})
	if len(report.findings) != 0 {
		t.Fatalf("checkRules findings = %v, want none (a symlink to a directory holds no file content)", report.findings)
	}
}

func TestRulesScanErrorNamesTheFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission checks are void")
	}
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	locked := writeTestFile(t, filepath.Join(root, "internal"), "locked.go", "package auth\n")
	git("add", "-A")
	git("commit", "-q", "-m", "add locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	report := checkRules(workingTreeRoots(root), filepath.Join(root, ".docs"), markerScanOptions{})
	if len(report.findings) != 1 {
		t.Fatalf("checkRules findings = %v, want exactly one scan error", report.findings)
	}
	got := report.findings[0]
	if got.file != "internal/locked.go" || got.line != 1 {
		t.Fatalf("finding at %s:%d, want internal/locked.go:1", got.file, got.line)
	}
	if !strings.Contains(got.message, "failed to scan") || !strings.Contains(got.message, "permission denied") {
		t.Fatalf("message = %q, want it to say the scan failed and why", got.message)
	}
}
