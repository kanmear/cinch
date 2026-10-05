package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"internal/sig/verify.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 1 || hits[0].kind != "owns" || hits[0].doc != ".docs/sig.md" ||
		len(hits[0].ruleIDs) != 1 || hits[0].ruleIDs[0] != "SIG-019" {
		t.Fatalf("hits = %+v, want one owns hit on .docs/sig.md naming SIG-019", hits)
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

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"internal/sig/verify.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 1 || hits[0].kind != "marker" || hits[0].ruleID != "SIG-020" {
		t.Fatalf("hits = %+v, want one marker hit on SIG-020", hits)
	}
	want := markerLoc{file: "internal/sig/verify.go", line: 3}
	if len(hits[0].markers) != 1 || hits[0].markers[0] != want {
		t.Fatalf("markers = %+v, want %+v", hits[0].markers, want)
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

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"internal/sig/verify.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}

	if len(hits) != 2 || hits[0].kind != "marker" || hits[0].ruleID != "SIG-020" ||
		hits[1].kind != "owns" || len(hits[1].ruleIDs) != 2 {
		t.Fatalf("hits = %+v, want a SIG-020 marker hit then one owns hit covering both doc rules", hits)
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

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"internal/unrelated/file.go"}, nil)
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

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"internal/sig/verify.go"}, nil)
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

// writeOwnsDoc writes a doc owning paths with n rules named prefix-001..n.
func writeOwnsDoc(t *testing.T, docs, name, prefix string, owns []string, n int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\nowns:\n")
	for _, o := range owns {
		b.WriteString("  - " + o + "\n")
	}
	b.WriteString("rule_prefix: " + prefix + "\n---\n# Doc\n\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%d. **%s-%03d** rule number %d\n", i, prefix, i, i)
	}
	writeTestFile(t, docs, name, b.String())
}

func TestImpactCollapsesPerDoc(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeOwnsDoc(t, docs, "auth.md", "AUTH", []string{"src/auth"}, 10)

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"src/auth/util.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 1 || len(hits[0].ruleIDs) != 10 {
		t.Fatalf("hits = %+v, want exactly one owns hit with 10 rules", hits)
	}
	out := captureStderr(t, func() { printImpact(hits) })
	if n := strings.Count(out, " owns "); n != 1 {
		t.Fatalf("output has %d owns lines, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "AUTH-001, AUTH-002, AUTH-003 +7 more") {
		t.Fatalf("output = %q, want 3 IDs and +7 more", out)
	}
}

func TestImpactAttributesByDocNotPrefix(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeOwnsDoc(t, docs, "auth.md", "AUTH", []string{"src/auth"}, 2)
	writeOwnsDoc(t, docs, "authz.md", "AUTHZ", []string{"src/authz"}, 2)

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"src/auth/x.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	out := captureStderr(t, func() { printImpact(hits) })
	if !strings.Contains(out, "AUTH-001") {
		t.Fatalf("output = %q, want AUTH rules named", out)
	}
	if strings.Contains(out, "AUTHZ-") || strings.Contains(out, "authz.md") {
		t.Fatalf("output = %q, must never mention AUTHZ", out)
	}
}

func TestImpactOwnsWithoutRulesIsSilent(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "empty.md", "---\nowns:\n  - src/auth\n---\n# No rules\n")

	hits, err := buildImpact(workingTreeRoots(root), docs, []string{"src/auth/x.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want none for an owning doc with no rules", hits)
	}
}

func TestImpactMarkerLineHasFileAndLine(t *testing.T) {
	hits := []impactHit{{kind: "marker", ruleID: "SIG-020", markers: []markerLoc{{file: "internal/sig/verify.go", line: 3}}}}
	out := captureStderr(t, func() { printImpact(hits) })
	if !strings.Contains(out, "SIG-020 — marker at internal/sig/verify.go:3") {
		t.Fatalf("output = %q, want file:line on the marker line", out)
	}
}

func TestPrintImpactGolden(t *testing.T) {
	hits := []impactHit{
		{kind: "marker", ruleID: "SIG-020", markers: []markerLoc{{file: "internal/sig/verify.go", line: 3}}},
		{kind: "marker", ruleID: "SIG-021", markers: []markerLoc{{file: "a.go", line: 1}, {file: "b.go", line: 9}}},
		{kind: "owns", doc: ".docs/auth.md",
			files:   []string{"src/auth/a.go", "src/auth/b.go", "src/auth/c.go", "src/auth/d.go"},
			ruleIDs: []string{"AUTH-001", "AUTH-002", "AUTH-003", "AUTH-004"}},
		{kind: "owns", doc: ".docs/sig.md", files: []string{"internal/sig/verify.go"}, ruleIDs: []string{"SIG-019"}},
	}
	got := captureStderr(t, func() { printImpact(hits) })
	want := "cinch impact: this diff plausibly touches:\n" +
		"  SIG-020 — marker at internal/sig/verify.go:3\n" +
		"  SIG-021 — marker at a.go:1, b.go:9\n" +
		"  .docs/auth.md — owns src/auth/a.go, src/auth/b.go, src/auth/c.go +1 more — rules AUTH-001, AUTH-002, AUTH-003 +1 more\n" +
		"  .docs/sig.md — owns internal/sig/verify.go — rules SIG-019\n" +
		"confirm the docs still match\n"
	if got != want {
		t.Fatalf("printImpact output:\n%s\nwant:\n%s", got, want)
	}
}
