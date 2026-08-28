package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImpactOwnsMatch(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "---\nowns:\n  - internal/sig\nrule_prefix: SIG-\n---\n"+
		"# Signatures\n\n1. **SIG-019** verify the signature\n")

	hits, err := buildImpact(root, docs, []string{"internal/sig/verify.go"})
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 1 || hits[0].ruleID != "SIG-019" || hits[0].reason != "owns" {
		t.Fatalf("hits = %+v, want one owns hit on SIG-019", hits)
	}
}

func TestImpactMarkerMatch(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "# Signatures\n\n1. **SIG-020** reject expired signatures\n")

	src := filepath.Join(root, "internal", "sig")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, "verify.go", "package sig\n\n// cinch:rule SIG-020\nfunc Verify() {}\n")

	hits, err := buildImpact(root, docs, []string{"internal/sig/verify.go"})
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 1 || hits[0].ruleID != "SIG-020" || hits[0].reason != "marker" {
		t.Fatalf("hits = %+v, want one marker hit on SIG-020", hits)
	}
}

func TestImpactBothInSameDiff(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "---\nowns:\n  - internal/sig\nrule_prefix: SIG-\n---\n"+
		"# Signatures\n\n1. **SIG-019** verify the signature\n2. **SIG-020** reject expired signatures\n")

	src := filepath.Join(root, "internal", "sig")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, "verify.go", "package sig\n\n// cinch:rule SIG-020\nfunc Verify() {}\n")

	hits, err := buildImpact(root, docs, []string{"internal/sig/verify.go"})
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}

	ids := map[string]bool{}
	for _, h := range hits {
		ids[h.ruleID] = true
	}
	if !ids["SIG-019"] || !ids["SIG-020"] {
		t.Fatalf("hits = %+v, want both SIG-019 (owns) and SIG-020 (marker) named", hits)
	}
}

func TestImpactNoMatchIsSilent(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "---\nowns:\n  - internal/sig\nrule_prefix: SIG-\n---\n"+
		"# Signatures\n\n1. **SIG-019** verify the signature\n")

	hits, err := buildImpact(root, docs, []string{"internal/unrelated/file.go"})
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want empty for an unrelated changed file", hits)
	}
	// printImpact on an empty slice must not panic and, by construction
	// (the len(hits)==0 early return), writes nothing.
	printImpact(hits)
}

func TestImpactPrefixNotGlob(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "---\nowns:\n  - internal/sig/*\nrule_prefix: SIG-\n---\n"+
		"# Signatures\n\n1. **SIG-019** verify the signature\n")

	hits, err := buildImpact(root, docs, []string{"internal/sig/verify.go"})
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want empty — owns: matching is prefix-only, a literal '*' never matches", hits)
	}
}

// TestCmdImpactSurfacesScanError confirms the standalone command is the
// opposite of the pre-commit hook's swallow-on-error behavior: here the
// user asked for this data directly, so a scan error must be loud.
func TestCmdImpactSurfacesScanError(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "malformed.md", "---\nowns: \"not-a-list\"\n---\n# Malformed\n\nno rules here\n")

	if got := CmdImpact(root, []string{"anything.go"}); got == 0 {
		t.Fatal("CmdImpact = 0, want a nonzero exit code for a malformed frontmatter scan error")
	}
}

func TestOwnsMatchesExactAndPrefix(t *testing.T) {
	owns := []string{"main.go", "internal/sig"}
	cases := []struct {
		changed string
		want    bool
	}{
		{"main.go", true},
		{"internal/sig/verify.go", true},
		{"internal/sig", true},
		{"internal/signature/verify.go", false}, // must not prefix-match a sibling dir
		{"other.go", false},
	}
	for _, tc := range cases {
		if got := ownsMatches(tc.changed, owns); got != tc.want {
			t.Errorf("ownsMatches(%q, %v) = %v, want %v", tc.changed, owns, got, tc.want)
		}
	}
}
