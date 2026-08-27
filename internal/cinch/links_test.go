package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkTargetIsExempt(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"", true},
		{"https://example.com", true},
		{"http://example.com/x", true},
		{"x://odd", true},
		{"#anchor", true},
		{"mailto:a@b.c", true},
		{"doc.md", false},
		{"../parent.md", false},
		{"./rel/path.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			if got := linkTargetIsExempt(tc.target); got != tc.want {
				t.Fatalf("linkTargetIsExempt(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}

// --- Seeded defect battery: links.go ---

func TestSeededDefectLinksTargetMoved(t *testing.T) {
	docsRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("[see](b.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// b.md intentionally does not exist — simulates a rename/move that left
	// the link behind.

	report := checkLinks(docsRoot)

	ok := false
	for _, f := range report.findings {
		if f.check == "links" && strings.Contains(f.message, "link target does not resolve: b.md") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'does not resolve' finding for b.md", report.findings)
	}
}

func TestSeededDefectLinksTargetResolves(t *testing.T) {
	docsRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("[see](b.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "b.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkLinks(docsRoot)

	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (link target exists)", report.findings)
	}
	if report.links != 1 {
		t.Fatalf("links = %d, want 1", report.links)
	}
}

func TestSeededDefectLinksTargetUnreachableFromRoot(t *testing.T) {
	docsRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# Root\n[see also](sub/b.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(docsRoot, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "sub", "b.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(docsRoot, "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	// c.md exists on disk and is walked/checked like any other doc, but
	// nothing anywhere links to it — it is unreachable from a.md (the
	// declared root) even though os.Stat happily resolves it.
	if err := os.WriteFile(filepath.Join(docsRoot, "orphan", "c.md"), []byte("# Orphan\nNothing links here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkLinks(docsRoot)

	// GAP: checkLinksInFile only performs os.Stat-based existence
	// resolution on each file's own outbound links — it has no
	// reachability-graph analysis, so a page that exists but is linked to
	// by nothing goes unnoticed. This fixture becomes the acceptance test
	// for the navigability check class described in
	// .docs/plans/05-navigability-check-class.md once it ships.
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (unreachable-but-existing pages are not detected)", report.findings)
	}
}
