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

func TestStripInlineCode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"no backticks", "plain [a](b.md)", "plain [a](b.md)"},
		{"single span", "x `[a](b)` y", "x `      ` y"},
		{"double span holds a single backtick", "`` a`b ``", "``" + "     " + "``"},
		{"longer run is not a closer", "`a``b`", "`    `"},
		{"unclosed run stays", "`[a](b.md)", "`[a](b.md)"},
		{"mismatched runs stay", "``a`", "``a`"},
		{"two spans", "`a` [l](m.md) `b`", "` ` [l](m.md) ` `"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripInlineCode(tc.line)
			if got != tc.want {
				t.Fatalf("stripInlineCode(%q) = %q, want %q", tc.line, got, tc.want)
			}
			if len(got) != len(tc.line) {
				t.Fatalf("length changed: %d -> %d", len(tc.line), len(got))
			}
		})
	}
}

func TestCheckLinksInlineCodeAndTitles(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantLinks int
	}{
		{"single-backtick span", "`[x](y)`\n", 0},
		{"double-backtick span", "`` [x](y) ``\n", 0},
		{"title in double quotes", "[a](real.md \"Title\")\n", 1},
		{"title in single quotes", "[a](real.md 'Title')\n", 1},
		{"angle-bracket target", "[a](<real.md>)\n", 1},
		{"unclosed backtick before a real link", "`oops [a](real.md)\n", 1},
		{"real link after a span", "`[x](y)` then [a](real.md)\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docsRoot := t.TempDir()
			writeTestFile(t, docsRoot, "real.md", "# Real\n")
			writeTestFile(t, docsRoot, "a.md", tc.content)

			report := checkLinks(docsRoot)

			if len(report.findings) != 0 {
				t.Fatalf("findings = %+v, want none", report.findings)
			}
			if report.links != tc.wantLinks {
				t.Fatalf("links = %d, want %d", report.links, tc.wantLinks)
			}
		})
	}
}

func TestCheckLinksUnclosedBacktickStillFlagsBrokenLink(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "`oops [a](missing.md)\n")

	report := checkLinks(docsRoot)

	if len(report.findings) != 1 || !strings.Contains(report.findings[0].message, "missing.md") {
		t.Fatalf("findings = %+v, want one 'does not resolve' finding for missing.md", report.findings)
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

	// Resolved (not a gap): .docs/plans/05-navigability-check-class.md
	// originally proposed a link-graph reachability pass rooted at a
	// declared entry doc (e.g. AGENTS.md). That was dropped — cinch doesn't
	// own AGENTS.md, and cinch's own entry point is `cinch index`, which
	// already lists every titled doc regardless of inbound links. So
	// orphan/c.md here is titled and correctly *not* a defect: it's
	// discoverable via `cinch index`. checkLinks only ever checks link
	// *existence*; index-completeness is checkIndex's job (index.go).
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (a titled doc with no inbound links is not a links-check defect)", report.findings)
	}
}

// --- Doc link graph ---

// docLinksFixture builds a corpus exercising every edge rule at once: a
// duplicated link, an anchored link, a self-link, a non-markdown target, and a
// target above the docs root.
func docLinksFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	docsRoot := filepath.Join(base, "docs")
	if err := os.MkdirAll(filepath.Join(docsRoot, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, base, "outside.md", "# Outside\n")
	writeTestFile(t, docsRoot, "notes.txt", "not markdown\n")
	writeTestFile(t, docsRoot, "a.md", "# A\n[x](sub/b.md)\n[y](./sub/b.md)\n[z](sub/b.md#anchor)\n[me](a.md)\n")
	writeTestFile(t, filepath.Join(docsRoot, "sub"), "b.md", "# B\n[up](../a.md)\n[code](../notes.txt)\n[out](../../outside.md)\n")
	writeTestFile(t, filepath.Join(docsRoot, "sub"), "c.md", "# C\n[b](b.md)\n")
	return docsRoot
}

func TestBuildDocLinks(t *testing.T) {
	graph := buildDocLinks(docLinksFixture(t))

	cases := []struct {
		name  string
		got   []string
		want  []string
		about string
	}{
		{"outbound a.md", graph.outbound["a.md"], []string{"sub/b.md"},
			"three links to the same target (one anchored) are one edge, and the self-link is dropped"},
		{"outbound sub/b.md", graph.outbound["sub/b.md"], []string{"a.md"},
			"../notes.txt is not markdown and ../../outside.md is above the docs root"},
		{"outbound sub/c.md", graph.outbound["sub/c.md"], []string{"sub/b.md"},
			"a sibling link resolves relative to its own directory"},
		{"inbound sub/b.md", graph.inbound["sub/b.md"], []string{"a.md", "sub/c.md"},
			"both referrers, sorted, regardless of how they spelled the path"},
		{"inbound a.md", graph.inbound["a.md"], []string{"sub/b.md"},
			"../a.md resolves back to the root doc"},
		{"inbound notes.txt", graph.inbound["notes.txt"], nil,
			"a non-markdown file is never a graph node"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Join(tc.got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("= %v, want %v (%s)", tc.got, tc.want, tc.about)
			}
		})
	}
}

// The links check and the graph read the same resolution, so a corpus that is
// green must not also be producing phantom edges — and vice versa.
func TestBuildDocLinksLeavesCheckIntact(t *testing.T) {
	report := checkLinks(docLinksFixture(t))

	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (every link in the fixture resolves)", report.findings)
	}
	if report.links != 8 {
		t.Fatalf("links = %d, want 8 (edge filtering must not change the link count)", report.links)
	}
}
