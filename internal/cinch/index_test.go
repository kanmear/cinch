package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckIndexMissingTitle(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "Just some prose, no heading.\n")

	report := checkIndex(docsRoot)

	if len(report.findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", report.findings)
	}
	if !strings.Contains(report.findings[0].message, "no title") {
		t.Fatalf("message = %q, want it to mention 'no title'", report.findings[0].message)
	}
}

func TestCheckIndexTitled(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "# A\nSome prose.\n")

	report := checkIndex(docsRoot)

	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (doc has a title)", report.findings)
	}
	if report.docs != 1 {
		t.Fatalf("docs = %d, want 1", report.docs)
	}
}

func TestCheckIndexExcludesPlans(t *testing.T) {
	docsRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(docsRoot, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(docsRoot, "plans"), "00-draft.md", "Untitled planning notes.\n")

	report := checkIndex(docsRoot)

	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (plans/ is excluded, same as indexList)", report.findings)
	}
	if report.docs != 0 {
		t.Fatalf("docs = %d, want 0 (plans/ doc should not be counted either)", report.docs)
	}
}

// --- Seeded defect battery: index.go ---

func TestSeededDefectIndexMissingH1(t *testing.T) {
	docsRoot := t.TempDir()
	// Looks like a complete doc — real content, just no leading "# " line.
	// This is the exact silent-drop scenario from the audit's F3 finding:
	// titleAndTrigger returns ("", ""), indexList skips it with no error,
	// and before this check there was no path to a finding at all.
	writeTestFile(t, docsRoot, "orphaned-heading.md", "## Not an H1\n\nThis doc has real content but no top-level heading.\n")

	report := checkIndex(docsRoot)

	ok := false
	for _, f := range report.findings {
		if f.check == "index" && strings.Contains(f.message, "silently dropped from cinch index") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a finding for the missing H1", report.findings)
	}
}
