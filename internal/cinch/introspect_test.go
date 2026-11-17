package cinch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCheckIndexMissingTitle(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "Just some prose, no heading.\n")

	report := checkIndex(workingTreeRoots(docsRoot), docsRoot)

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

	report := checkIndex(workingTreeRoots(docsRoot), docsRoot)

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

	report := checkIndex(workingTreeRoots(docsRoot), docsRoot)

	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (plans/ is excluded, same as indexList)", report.findings)
	}
	if report.docs != 0 {
		t.Fatalf("docs = %d, want 0 (plans/ doc should not be counted either)", report.docs)
	}
}

// --- Seeded defect battery: index.go ---

// catch-matrix: index | caught | doc with no H1, silently dropped from cinch index
func TestSeededDefectIndexMissingH1(t *testing.T) {
	docsRoot := t.TempDir()
	// Looks like a complete doc — real content, just no leading "# " line.
	// This is the exact silent-drop scenario from the audit's F3 finding:
	// titleAndTrigger returns ("", ""), indexList skips it with no error,
	// and before this check there was no path to a finding at all.
	writeTestFile(t, docsRoot, "orphaned-heading.md", "## Not an H1\n\nThis doc has real content but no top-level heading.\n")

	report := checkIndex(workingTreeRoots(docsRoot), docsRoot)

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

// --- Link queries ---

// collidingDocsFixture reproduces the shape that motivated the link graph: one
// basename naming three different docs, referenced through three different
// relative spellings.
func collidingDocsFixture(t *testing.T) string {
	t.Helper()
	docsRoot := t.TempDir()
	for _, sub := range []string{"backend", "frontend"} {
		if err := os.MkdirAll(filepath.Join(docsRoot, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, docsRoot, "architecture.md", "# Architecture\n[be](backend/architecture.md)\n[fe](frontend/architecture.md)\n")
	writeTestFile(t, filepath.Join(docsRoot, "backend"), "architecture.md", "# Backend Architecture\n[up](../architecture.md)\n")
	writeTestFile(t, filepath.Join(docsRoot, "backend"), "testing.md", "# Backend Testing\n[arch](./architecture.md)\n")
	writeTestFile(t, filepath.Join(docsRoot, "frontend"), "architecture.md", "# Frontend Architecture\n")
	return docsRoot
}

func TestMatchDocPaths(t *testing.T) {
	titles, err := docTitleByPath(collidingDocsFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		target string
		want   []string
	}{
		{"bare basename returns every collision", "architecture.md",
			[]string{"architecture.md", "backend/architecture.md", "frontend/architecture.md"}},
		{"a path with a directory is an address", "backend/architecture.md",
			[]string{"backend/architecture.md"}},
		{"leading ./ is stripped", "./backend/architecture.md",
			[]string{"backend/architecture.md"}},
		{"an unambiguous basename resolves alone", "testing.md",
			[]string{"backend/testing.md"}},
		{"no match returns nothing", "nope.md", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchDocPaths(titles, tc.target)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("matchDocPaths(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}

func TestLinksListDisambiguates(t *testing.T) {
	list, err := linksList(collidingDocsFixture(t), "architecture.md", true)
	if err != nil {
		t.Fatal(err)
	}

	// The whole point: a bare colliding basename must not silently answer for
	// the one at the docs root.
	for _, want := range []string{
		`3 docs match "architecture.md"`,
		"architecture.md — Architecture (1 in, 2 out)",
		"backend/architecture.md — Backend Architecture (2 in, 1 out)",
		"frontend/architecture.md — Frontend Architecture (1 in, 0 out)",
		"to see what links to it",
	} {
		if !strings.Contains(list, want) {
			t.Fatalf("list = %q, want it to contain %q", list, want)
		}
	}

	// Both directions list the same candidates, so the closing line is the
	// only thing distinguishing them — it must not come back identical.
	outbound, err := linksList(collidingDocsFixture(t), "architecture.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(outbound, "to see what it links to") {
		t.Fatalf("outbound = %q, want it to close on the outbound question", outbound)
	}
	if outbound == list {
		t.Fatal("--links-to and --links-from printed identical disambiguation listings")
	}
}

func TestLinksListResolvesRelativeSpellings(t *testing.T) {
	docsRoot := collidingDocsFixture(t)

	// backend/architecture.md is reached as "backend/architecture.md" from the
	// root doc and as "./architecture.md" from its own directory; both are the
	// same edge target, and neither of the other two architecture.md files is.
	inbound, err := linksList(docsRoot, "backend/architecture.md", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2 docs link to backend/architecture.md", "architecture.md — Architecture", "backend/testing.md — Backend Testing"} {
		if !strings.Contains(inbound, want) {
			t.Fatalf("inbound = %q, want it to contain %q", inbound, want)
		}
	}

	outbound, err := linksList(docsRoot, "frontend/architecture.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(outbound, "frontend/architecture.md links to no docs") {
		t.Fatalf("outbound = %q, want the no-outbound-links message", outbound)
	}
}

func TestLinksListNoInboundAndNoMatch(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "lonely.md", "# Lonely\n")

	list, err := linksList(docsRoot, "lonely.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "nothing links to lonely.md") {
		t.Fatalf("list = %q, want the nothing-links-here message", list)
	}

	if _, err := linksList(docsRoot, "ghost.md", true); err == nil {
		t.Fatal("linksList on an unknown doc = nil error, want a 'matches no doc' error")
	}
}

func TestLinksListAcceptsDocsRootPrefix(t *testing.T) {
	docsRoot := collidingDocsFixture(t)

	// A path pasted out of an editor or a cinch finding carries the docs root.
	list, err := linksList(docsRoot, filepath.Join(docsRoot, "backend", "architecture.md"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "2 docs link to backend/architecture.md") {
		t.Fatalf("list = %q, want the docs-root-prefixed path to resolve", list)
	}
}

func TestLinksListAgreesVerbWithCount(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "# A\n\n[b](b.md)\n")
	writeTestFile(t, docsRoot, "b.md", "# B\n")
	writeTestFile(t, docsRoot, "c.md", "# C\n\n[b](b.md)\n")

	one, err := linksList(docsRoot, "a.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(one, "a.md links to 1 doc:") {
		t.Fatalf("outbound = %q, want %q", one, "a.md links to 1 doc:")
	}
	oneInbound, err := linksList(docsRoot, "a.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(oneInbound, "nothing links to a.md") {
		t.Fatalf("inbound = %q, want the nothing-links message", oneInbound)
	}

	two, err := linksList(docsRoot, "b.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(two, "2 docs link to b.md:") {
		t.Fatalf("inbound = %q, want %q", two, "2 docs link to b.md:")
	}

	writeTestFile(t, docsRoot, "c.md", "# C\n")
	single, err := linksList(docsRoot, "b.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(single, "1 doc links to b.md:") {
		t.Fatalf("inbound = %q, want %q", single, "1 doc links to b.md:")
	}
}

func workflowsFixture(t *testing.T) string {
	t.Helper()
	docsRoot := t.TempDir()
	workflows := filepath.Join(docsRoot, workflowsSubdir)
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, workflows, "dev-fix-bug.md", "# Bug Fix\n\nUse this when a bug crosses layers, needs more than one session, or comes from a missing rule.\n")
	writeTestFile(t, workflows, "docs.md", "# Docs\n\nUse this to sync docs.\n")
	return docsRoot
}

func TestWorkflowsTablePipedIsMarkdown(t *testing.T) {
	docsRoot := workflowsFixture(t)

	// The width is ignored when piped: agents parse this table.
	for _, width := range []int{20, 100} {
		got, err := workflowsTable(docsRoot, false, width)
		if err != nil {
			t.Fatal(err)
		}
		want := "2 workflows:\n\n" +
			"| Workflow | Trigger |\n" +
			"|---|---|\n" +
			"| [dev-fix-bug.md](dev-fix-bug.md) | Use this when a bug crosses layers, needs more than one session, or comes from a missing rule. |\n" +
			"| [docs.md](docs.md) | Use this to sync docs. |\n"
		if got != want {
			t.Fatalf("width %d: table =\n%s\nwant\n%s", width, got, want)
		}
	}
}

func TestWorkflowsTableTerminalAlignsAndWraps(t *testing.T) {
	docsRoot := workflowsFixture(t)

	got, err := workflowsTable(docsRoot, true, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := "2 workflows:\n\n" +
		"dev-fix-bug  Use this when a bug crosses layers,\n" +
		"             needs more than one session, or comes\n" +
		"             from a missing rule.\n" +
		"docs         Use this to sync docs.\n"
	if got != want {
		t.Fatalf("table =\n%s\nwant\n%s", got, want)
	}
}

func TestWrapWords(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"fits", "one two", 20, []string{"one two"}},
		{"wraps at a space", "one two three", 7, []string{"one two", "three"}},
		{"long word keeps its own line", "a bigword b", 3, []string{"a", "bigword", "b"}},
		{"empty", "", 10, []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrapWords(tc.text, tc.width); !slices.Equal(got, tc.want) {
				t.Fatalf("wrapWords(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
			}
		})
	}
}

func TestIndexListAlignsPathsOnlyOnATerminal(t *testing.T) {
	docsRoot := t.TempDir()
	writeTestFile(t, docsRoot, "a.md", "# Alpha\n")
	if err := os.MkdirAll(filepath.Join(docsRoot, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(docsRoot, "workflows"), "long-name.md", "# Long\n")

	piped, err := indexList(docsRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2 docs under " + docsRoot + ":\n\na.md — Alpha\nworkflows/long-name.md — Long\n"; piped != want {
		t.Fatalf("piped index =\n%s\nwant\n%s", piped, want)
	}

	aligned, err := indexList(docsRoot, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2 docs under " + docsRoot + ":\n\na.md                   — Alpha\nworkflows/long-name.md — Long\n"; aligned != want {
		t.Fatalf("aligned index =\n%s\nwant\n%s", aligned, want)
	}
}

func TestPhilosophyTriggerNamesItsRole(t *testing.T) {
	title, trigger := titleAndTrigger(philosophySrc)
	if title == "" || !strings.HasPrefix(trigger, "Use this as the reference the other workflows cite") {
		t.Fatalf("title = %q, trigger = %q, want a 'Use this ...' trigger like the other workflows", title, trigger)
	}
}
