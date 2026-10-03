package cinch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseRuleItemsStr(t *testing.T, body string) []ruleItem {
	t.Helper()
	return parseRuleItems("test.md", []byte(body))
}

// unreadableTree returns a temp dir whose sub/ subtree is unreadable,
// skipping the test when running as root (permission checks are void).
func unreadableTree(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission checks are void")
	}
	directory := t.TempDir()
	sub := filepath.Join(directory, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(sub, 0o755); err != nil {
			t.Logf("restoring permissions on %s: %v", sub, err)
		}
	})
	return directory
}

func TestParseRuleItemsBasic(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** first rule\n2. **DEF-2** second rule\n")
	want := []ruleItem{
		{id: "ABC-1", file: "test.md", line: 1, text: "1. **ABC-1** first rule"},
		{id: "DEF-2", file: "test.md", line: 2, text: "2. **DEF-2** second rule"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseRuleItems = %v, want %v", got, want)
	}
}

func TestParseRuleItemsMultiParagraphText(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** first line\n   continuation\n\n   second paragraph\n")
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	wantText := "1. **ABC-1** first line continuation second paragraph"
	if got[0].text != wantText {
		t.Fatalf("text = %q, want %q", got[0].text, wantText)
	}
}

func TestParseRuleItemsSkipsFencedCode(t *testing.T) {
	body := "1. **ABC-1** visible\n```\n1. **FAKE-9** hidden\n```\n"
	got := parseRuleItemsStr(t, body)
	if len(got) != 1 || got[0].id != "ABC-1" {
		t.Fatalf("parseRuleItems = %v, want only ABC-1", got)
	}
}

func TestParseRuleItemsCRLF(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** rule\r\n2. **DEF-2** next\r\n")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].line != 1 || got[1].line != 2 {
		t.Fatalf("line numbers = %d,%d, want 1,2", got[0].line, got[1].line)
	}
	if got[0].text != "1. **ABC-1** rule" {
		t.Fatalf("text = %q, want no carriage returns", got[0].text)
	}
}

func TestParseRuleItemsIgnore(t *testing.T) {
	t.Run("no reason", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n<!-- cinch:ignore -->\n")
		if len(got) != 1 || !got[0].hasIgnore || got[0].ignoreReason != "" || got[0].ignoreLine != 2 {
			t.Fatalf("item = %+v, want hasIgnore with empty reason at line 2", got[0])
		}
	})

	t.Run("with reason", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n<!-- cinch:ignore: historical debt -->\n")
		if len(got) != 1 || !got[0].hasIgnore || got[0].ignoreReason != "historical debt" {
			t.Fatalf("item = %+v, want hasIgnore with reason", got[0])
		}
	})

	t.Run("inside fence ignored", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n```\n<!-- cinch:ignore -->\n```\n")
		if len(got) != 1 || got[0].hasIgnore {
			t.Fatalf("item = %+v, want hasIgnore false", got[0])
		}
	})
}

func TestParseRuleItemsPlainNumberedItemFlushes(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** rule\n2. plain item\n3. **DEF-2** next\n")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].id != "ABC-1" || got[0].text != "1. **ABC-1** rule" {
		t.Fatalf("first item = %+v, want ABC-1 without trailing plain item text", got[0])
	}
	if got[1].id != "DEF-2" {
		t.Fatalf("second item = %+v, want DEF-2", got[1])
	}
}

func TestParseRuleItemsNoRules(t *testing.T) {
	if got := parseRuleItemsStr(t, "just text\nno rules here\n"); len(got) != 0 {
		t.Fatalf("parseRuleItems = %v, want none", got)
	}
}

func TestParseRuleDocReadError(t *testing.T) {
	if _, err := parseRuleDoc(filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Fatal("parseRuleDoc = nil error, want error for unreadable file")
	}
}

func TestScanRuleDocsPropagatesWalkError(t *testing.T) {
	root := unreadableTree(t)
	if _, err := scanRuleDocs(root); err == nil {
		t.Fatal("scanRuleDocs = nil error, want error for unreadable subdirectory")
	}
}

func TestScanRuleMarkersReadError(t *testing.T) {
	root := t.TempDir()
	broken := filepath.Join(root, "broken.go")
	if err := os.Symlink("missing.go", broken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), nil); err == nil {
		t.Fatal("scanRuleMarkers = nil error, want error for unreadable file")
	}
}

func TestCheckRulesReportsScanError(t *testing.T) {
	docsRoot := unreadableTree(t)
	report := checkRules(workingTreeRoots("."), docsRoot, nil)
	if len(report.findings) == 0 {
		t.Fatal("checkRules = no findings, want error finding for unreadable subdirectory")
	}
	ok := false
	for _, f := range report.findings {
		if f.check == "rules" && f.level == "error" && strings.Contains(f.message, "failed to scan") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("checkRules findings %+v do not include a scan error", report.findings)
	}
}

func TestCheckRulesScanErrorDoesNotReportUnmarked(t *testing.T) {
	repoRoot := t.TempDir()
	docsRoot := filepath.Join(repoRoot, "docs")
	if err := os.Mkdir(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **ABC-1** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A broken symlink makes the marker scan fail after the docs scan
	// succeeds, isolating the marker-scan error path.
	if err := os.Symlink(filepath.Join(repoRoot, "missing.go"), filepath.Join(repoRoot, "broken.go")); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(repoRoot), docsRoot, nil)
	scanErr := false
	unmarked := 0
	for _, f := range report.findings {
		if f.check == "rules" && strings.Contains(f.message, "failed to scan") {
			scanErr = true
		}
		if f.check == "rules" && strings.Contains(f.message, "no // cinch:rule marker") {
			unmarked++
		}
	}
	if !scanErr {
		t.Fatalf("checkRules findings %+v do not include a scan error", report.findings)
	}
	if unmarked > 0 {
		t.Fatalf("scan failure leaked %d false 'unmarked' findings; want none", unmarked)
	}
}

func TestScanRuleMarkersLargeFile(t *testing.T) {
	root := t.TempDir()
	line := strings.Repeat("// filler ", 128) + "\n"
	var b strings.Builder
	for b.Len() < 5<<20 {
		b.WriteString(line)
	}
	b.WriteString("// cinch:rule ABC-1\n")
	if err := os.WriteFile(filepath.Join(root, "big.go"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	markers, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), nil)
	if err != nil {
		t.Fatalf("scanRuleMarkers = %v, want nil (a >4MB file must not abort the scan)", err)
	}
	if len(markers["ABC-1"]) != 1 {
		t.Fatalf("markers[ABC-1] = %v, want 1 loc in the large file", markers["ABC-1"])
	}
}

func TestScanMarkersFenceOnlyForMarkdown(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("note.md", "before\n```\n// cinch:rule ABC-1\n```\nafter\n// cinch:rule DEF-2\n")
	write("x.go", "```\n// cinch:rule ABC-2\n")

	markers, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), nil)
	if err != nil {
		t.Fatalf("scanRuleMarkers = %v, want nil", err)
	}
	if len(markers["ABC-1"]) != 0 {
		t.Fatalf("ABC-1 inside a .md fence must be skipped, got %v", markers["ABC-1"])
	}
	if len(markers["DEF-2"]) != 1 {
		t.Fatalf("DEF-2 outside a .md fence must be found, got %v", markers["DEF-2"])
	}
	if len(markers["ABC-2"]) != 1 {
		t.Fatalf("ABC-2 in a .go file after a ``` line must be found, got %v", markers["ABC-2"])
	}
}

func TestMarkerReCommentLeaders(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"double slash", "// cinch:rule ABC-1", "ABC-1"},
		{"hash", "# cinch:rule ABC-1", "ABC-1"},
		{"double dash", "-- cinch:rule ABC-1", "ABC-1"},
		{"html comment", "<!-- cinch:rule ABC-1 -->", "ABC-1"},
		{"block comment", "/* cinch:rule ABC-1 */", "ABC-1"},
		{"percent", "% cinch:rule ABC-1", "ABC-1"},
		{"semicolon", "; cinch:rule ABC-1", "ABC-1"},
		{"decrement, no false positive", "i--;", ""},
		{"markdown rule, no false positive", "---", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := markerRe.FindStringSubmatch(tc.line)
			if tc.want == "" {
				if m != nil {
					t.Fatalf("markerRe matched %q, want no match", tc.line)
				}
				return
			}
			if m == nil || m[1] != tc.want {
				t.Fatalf("markerRe.FindStringSubmatch(%q) = %v, want ID %q", tc.line, m, tc.want)
			}
		})
	}
}

func TestScanRuleMarkersGitScope(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	write(".gitignore", "vendor/\n")
	write("a.go", "// cinch:rule ABC-1\n")
	write("vendor/skip.go", "// cinch:rule ABC-2\n")
	write("vendor/kept.go", "// cinch:rule ABC-3\n")
	git("add", "a.go", ".gitignore")
	git("add", "-f", "vendor/kept.go")

	markers, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), nil)
	if err != nil {
		t.Fatalf("scanRuleMarkers = %v, want nil", err)
	}
	if len(markers["ABC-1"]) != 1 {
		t.Fatalf("tracked marker ABC-1 = %v, want 1 loc", markers["ABC-1"])
	}
	if len(markers["ABC-2"]) != 0 {
		t.Fatalf("ignored marker ABC-2 = %v, want none (gitignored file is out of scope)", markers["ABC-2"])
	}
	if len(markers["ABC-3"]) != 1 {
		t.Fatalf("force-added marker ABC-3 = %v, want 1 loc (tracked despite ignore)", markers["ABC-3"])
	}
}

func TestRulesCheckResultDetailAndNoOp(t *testing.T) {
	empty := rulesCheckResult(rulesReport{})
	if empty.noOp == "" {
		t.Fatalf("rulesCheckResult(empty) = %+v, want noOp set for zero rules and zero findings", empty)
	}

	report := rulesReport{rules: 8, docs: 1, ignores: 1}
	result := rulesCheckResult(report)
	if result.noOp != "" {
		t.Fatalf("rulesCheckResult(%+v).noOp = %q, want empty", report, result.noOp)
	}
	if want := "(8 rules, 1 rule doc, 1 ignore)"; result.detail != want {
		t.Fatalf("rulesCheckResult(%+v).detail = %q, want %q", report, result.detail, want)
	}

	strayMarker := rulesReport{findings: []finding{{check: "rules", message: "stray marker"}}}
	result = rulesCheckResult(strayMarker)
	if result.noOp != "" {
		t.Fatalf("rulesCheckResult(%+v).noOp = %q, want empty (findings must not be swallowed into noOp)", strayMarker, result.noOp)
	}
	if len(result.findings) != 1 {
		t.Fatalf("rulesCheckResult(%+v).findings = %+v, want the stray-marker finding preserved", strayMarker, result.findings)
	}
}

func TestLinksCheckResultDetailAndNoOp(t *testing.T) {
	empty := linksCheckResult(linksReport{})
	if empty.noOp == "" {
		t.Fatalf("linksCheckResult(empty) = %+v, want noOp set for zero docs and zero findings", empty)
	}

	report := linksReport{docs: 4, links: 12}
	result := linksCheckResult(report)
	if result.noOp != "" {
		t.Fatalf("linksCheckResult(%+v).noOp = %q, want empty", report, result.noOp)
	}
	if want := "(4 docs, 12 links checked)"; result.detail != want {
		t.Fatalf("linksCheckResult(%+v).detail = %q, want %q", report, result.detail, want)
	}
}

func TestNearMissRuleIDs(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		ruleIDs map[string]bool
		want    []string
	}{
		{"transposition", "SIG-091", map[string]bool{"SIG-019": true}, []string{"SIG-019"}},
		{"truncation", "SIG-01", map[string]bool{"SIG-019": true}, []string{"SIG-019"}},
		{"different prefix, no false positive", "TAB-019", map[string]bool{"SIG-019": true}, nil},
		{
			"caps at 2 closest candidates", "SIG-01",
			map[string]bool{"SIG-010": true, "SIG-011": true, "SIG-012": true},
			[]string{"SIG-010", "SIG-011"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nearMissRuleIDs(tc.id, tc.ruleIDs)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("nearMissRuleIDs(%q, %v) = %v, want %v", tc.id, tc.ruleIDs, got, tc.want)
			}
		})
	}
}

func TestCheckRulesFromNearMissSuggestion(t *testing.T) {
	items := parseRuleItemsStr(t, "1. **SIG-019** real rule\n")
	markers := map[string][]markerLoc{"SIG-091": {{file: "a.go", line: 5}}}

	report := checkRulesFrom(items, markers)

	if len(report.findings) != 2 {
		t.Fatalf("findings = %+v, want 2 (unmarked SIG-019 + unresolved SIG-091)", report.findings)
	}
	wantSuggestion := "// cinch:rule SIG-091 does not resolve to any rule ID (did you mean SIG-019?)"
	found := false
	for _, f := range report.findings {
		if f.file == "a.go" && f.line == 5 {
			if f.message != wantSuggestion {
				t.Fatalf("message = %q, want %q", f.message, wantSuggestion)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("findings %+v do not include the unresolved-marker finding", report.findings)
	}
}

func TestCheckRulesFromDuplicateID(t *testing.T) {
	items := []ruleItem{
		{id: "SIG-019", file: "domain/signatures.md", line: 12},
		{id: "SIG-019", file: "domain/tabs.md", line: 44},
	}
	markers := map[string][]markerLoc{"SIG-019": {{file: "a.go", line: 1}}}

	report := checkRulesFrom(items, markers)

	want := "SIG-019: declared in two places (domain/signatures.md:12 and domain/tabs.md:44) — rule IDs must be unique"
	if len(report.findings) != 1 || report.findings[0].message != want {
		t.Fatalf("findings = %+v, want single finding %q", report.findings, want)
	}
}

func TestCheckRulesFromNoDuplicateForDistinctIDs(t *testing.T) {
	items := []ruleItem{
		{id: "SIG-001", file: "a.md", line: 1},
		{id: "SIG-002", file: "b.md", line: 2},
	}
	markers := map[string][]markerLoc{
		"SIG-001": {{file: "a.go", line: 1}},
		"SIG-002": {{file: "b.go", line: 2}},
	}

	report := checkRulesFrom(items, markers)

	for _, f := range report.findings {
		if strings.Contains(f.message, "declared in") {
			t.Fatalf("findings = %+v, want no duplicate-ID finding for distinct IDs", report.findings)
		}
	}
}

func TestCheckRulesFromTripleDuplicate(t *testing.T) {
	items := []ruleItem{
		{id: "SIG-005", file: "a.md", line: 1},
		{id: "SIG-005", file: "b.md", line: 2},
		{id: "SIG-005", file: "c.md", line: 3},
	}
	markers := map[string][]markerLoc{"SIG-005": {{file: "x.go", line: 1}}}

	report := checkRulesFrom(items, markers)

	want := "SIG-005: declared in 3 places (a.md:1, b.md:2, c.md:3) — rule IDs must be unique"
	if len(report.findings) != 1 || report.findings[0].message != want {
		t.Fatalf("findings = %+v, want single finding %q", report.findings, want)
	}
}

// --- Seeded defect battery: rules.go ---
//
// Each test below pairs a minimal fixture with the finding(s) cinch check
// should (or, for documented gaps, currently does not) report. Together
// with the links/generated battery cases in their own files, this is the
// checker's own regression suite — see
// .docs/plans/03-rule-grammar-holes-and-seed-defects.md.

func TestSeededDefectRulesMarkerDeleted(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** some rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	if len(report.findings) != 1 || !strings.Contains(report.findings[0].message, "no // cinch:rule marker") {
		t.Fatalf("findings = %+v, want single 'no marker' finding", report.findings)
	}
}

func TestSeededDefectRulesHollowedTest(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** some rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "// cinch:rule SIG-001\nfunc TestX(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(root, "a_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	// GAP: the marker resolves to a real rule ID, so the grammar check
	// passes even though the test body it sits next to has been gutted.
	// cinch has no structural way to detect this (it's the T3 construction
	// from the benchmark); documented here as a known, deliberate gap.
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (hollowed test bodies are not structurally detectable)", report.findings)
	}
}

func TestSeededDefectRulesContradictingNeighbor(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "1. **SIG-001** applies to all files under src/\n2. **SIG-002** applies to all files under src/\n"
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("// cinch:rule SIG-001\n// cinch:rule SIG-002\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	// GAP: rules.go only checks marker presence, ID resolution, and ignore
	// hygiene — it has no prose-semantics analysis, so two rules that
	// claim overlapping/contradicting scope go unnoticed.
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (prose contradictions are not checked)", report.findings)
	}
}

func TestSeededDefectRulesDuplicateID(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(filepath.Join(docsRoot, "domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "domain", "signatures.md"), []byte("1. **SIG-019** rule text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "domain", "tabs.md"), []byte("1. **SIG-019** other rule text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("// cinch:rule SIG-019\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	ok := false
	for _, f := range report.findings {
		if strings.Contains(f.message, "declared in two places") &&
			strings.Contains(f.message, "signatures.md") && strings.Contains(f.message, "tabs.md") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a duplicate-ID finding naming both files", report.findings)
	}
}

func TestSeededDefectRulesNearMissID(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-019** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("// cinch:rule SIG-091\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	ok := false
	for _, f := range report.findings {
		if strings.Contains(f.message, "does not resolve to any rule ID (did you mean SIG-019?)") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a near-miss suggestion for SIG-091", report.findings)
	}
}

func TestSeededDefectRulesIgnoreNoReason(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "1. **SIG-001** rule\n<!-- cinch:ignore -->\n"
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	ok := false
	for _, f := range report.findings {
		if strings.Contains(f.message, "cinch:ignore has no reason") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'no reason' finding", report.findings)
	}
}

func TestSeededDefectRulesIgnoreWithMarker(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "1. **SIG-001** rule\n<!-- cinch:ignore: reason -->\n"
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("// cinch:rule SIG-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(root), docsRoot, nil)

	ok := false
	for _, f := range report.findings {
		if strings.Contains(f.message, "declared cinch:ignore but also has a // cinch:rule marker") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'pick one' finding", report.findings)
	}
}

// --- rules.roots: sibling code repos for a docs-only repo ---

func TestScanRuleMarkersPrefixesSiblingRootFindings(t *testing.T) {
	repoRoot := t.TempDir()
	sibling := t.TempDir()
	if err := os.WriteFile(filepath.Join(sibling, "a.go"), []byte("// cinch:rule SIG-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(repoRoot, sibling)
	if err != nil {
		t.Fatal(err)
	}

	markers, missing, err := scanRuleMarkers(workingTreeRoots(repoRoot), filepath.Join(repoRoot, "docs"), []string{rel})
	if err != nil {
		t.Fatalf("scanRuleMarkers: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (sibling root exists)", missing)
	}
	locs := markers["SIG-001"]
	if len(locs) != 1 {
		t.Fatalf("markers[SIG-001] = %v, want 1 loc", locs)
	}
	wantFile := filepath.ToSlash(filepath.Join(rel, "a.go"))
	if locs[0].file != wantFile {
		t.Fatalf("file = %q, want %q (prefixed with the configured sibling root)", locs[0].file, wantFile)
	}
}

// git quotes a non-ASCII path in plain ls-files output, which used to make
// the marker scan open a file that doesn't exist and fail the whole check.
func TestCheckRulesFindsMarkerInNonASCIIPath(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	for _, directory := range []string{"docs", "internal"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "docs"), "rules.md", "1. **AUTH-001** tokens rotate\n")
	writeTestFile(t, filepath.Join(root, "internal"), "rötate.go", "// cinch:rule AUTH-001\n")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	report := checkRules(workingTreeRoots(root), filepath.Join(root, "docs"), nil)
	if len(report.findings) != 0 || report.rules != 1 {
		t.Fatalf("report = %+v, want 1 rule and no findings (AUTH-001 is marked in internal/rötate.go)", report)
	}
}

// A hook spawned by `git commit -a` inherits an absolute GIT_INDEX_FILE for
// the primary repo. Listing a sibling repo's files with it would return the
// primary's index entries, which don't exist in the sibling and fail the scan.
func TestScanRuleMarkersSiblingIgnoresInheritedIndex(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "sib")
	for _, directory := range []string{repoRoot, sibling} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitRepo := initTestGitRepo(t, repoRoot)
	writeTestFile(t, repoRoot, "only-in-repo.go", "package main\n")
	gitRepo("add", "-A")
	gitRepo("commit", "-q", "-m", "seed")
	gitSibling := initTestGitRepo(t, sibling)
	writeTestFile(t, sibling, "a.go", "// cinch:rule SIG-001\n")
	gitSibling("add", "-A")
	gitSibling("commit", "-q", "-m", "seed")

	t.Setenv("GIT_INDEX_FILE", filepath.Join(repoRoot, ".git", "index"))

	markers, _, err := scanRuleMarkers(workingTreeRoots(repoRoot), filepath.Join(repoRoot, "docs"), []string{"../sib"})
	if err != nil {
		t.Fatalf("scanRuleMarkers: %v", err)
	}
	if locs := markers["SIG-001"]; len(locs) != 1 || locs[0].file != "../sib/a.go" {
		t.Fatalf("markers[SIG-001] = %v, want one loc at ../sib/a.go", locs)
	}
}

func TestCheckRulesScansSiblingRoot(t *testing.T) {
	repoRoot := t.TempDir()
	docsRoot := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := t.TempDir()
	if err := os.WriteFile(filepath.Join(sibling, "a.go"), []byte("// cinch:rule SIG-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(repoRoot, sibling)
	if err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(repoRoot), docsRoot, []string{rel})
	if report.skip != "" {
		t.Fatalf("skip = %q, want empty (sibling root is present)", report.skip)
	}
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (marker found in the sibling root)", report.findings)
	}
}

func TestCheckRulesSkipsWhenAllRootsMissing(t *testing.T) {
	repoRoot := t.TempDir()
	docsRoot := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(repoRoot), docsRoot, []string{"../does-not-exist", "../also-missing"})
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none — every configured root is absent, so this should skip "+
			"rather than report a false 'no marker found' per rule", report.findings)
	}
	if report.skip == "" {
		t.Fatal("skip = \"\", want a skip message naming the missing roots")
	}
	if !strings.Contains(report.skip, "does-not-exist") || !strings.Contains(report.skip, "also-missing") {
		t.Fatalf("skip = %q, want it to name both missing roots", report.skip)
	}
}

func TestCheckRulesPartialRootsStillChecksPresentOnes(t *testing.T) {
	repoRoot := t.TempDir()
	docsRoot := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := t.TempDir()
	if err := os.WriteFile(filepath.Join(sibling, "a.go"), []byte("// cinch:rule SIG-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(repoRoot, sibling)
	if err != nil {
		t.Fatal(err)
	}

	report := checkRules(workingTreeRoots(repoRoot), docsRoot, []string{rel, "../does-not-exist"})
	if report.skip != "" {
		t.Fatalf("skip = %q, want empty — one of the two configured roots is present", report.skip)
	}
	if len(report.findings) != 0 {
		t.Fatalf("findings = %+v, want none (marker found in the present sibling root)", report.findings)
	}
}

func TestCheckRulesUnaffectedWithoutConfiguredRoots(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "rules.md"), []byte("1. **SIG-001** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No extraRoots at all (nil, today's only mode): a real gap must still
	// report normally, not get swallowed by the new skip path.
	report := checkRules(workingTreeRoots(root), docsRoot, nil)
	if report.skip != "" {
		t.Fatalf("skip = %q, want empty when rules.roots is unset entirely", report.skip)
	}
	if len(report.findings) != 1 || !strings.Contains(report.findings[0].message, "no // cinch:rule marker") {
		t.Fatalf("findings = %+v, want a single 'no marker' finding", report.findings)
	}
}

// TestMarkerScanSkipsSubmoduleGitlink pins the fix for a real bug found while adapting
// fsed-odin-docs to use git submodules: git ls-files lists a submodule as one gitlink
// entry — a path that is actually a directory on disk, not a blob — and opening it as a
// file used to succeed (Linux permits open() on a directory) but fail on the first read
// with EISDIR, surfacing as a bogus "failed to scan" finding. The primary scan must skip
// it silently; rules.roots remains the supported way to also scan a submodule's content.
func TestMarkerScanSkipsSubmoduleGitlink(t *testing.T) {
	subSrc := t.TempDir()
	subGit := gitTestHelper(t, subSrc)
	subGit("init", "-q")
	subGit("config", "user.email", "cinch@test")
	subGit("config", "user.name", "cinch test")
	writeTestFile(t, subSrc, "a.go", "// cinch:rule ABC-1\n")
	subGit("add", "a.go")
	subGit("commit", "-q", "-m", "seed")

	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	git("-c", "protocol.file.allow=always", "submodule", "add", "-q", subSrc, "sub")

	// Primary scan (no rules.roots): the submodule gitlink must be skipped, not opened.
	markers, missing, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), nil)
	if err != nil {
		t.Fatalf("scanRuleMarkers = %v, want nil (submodule gitlink must be skipped, not opened as a file)", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(markers["ABC-1"]) != 0 {
		t.Fatalf("markers[ABC-1] via primary scan = %v, want none (only rules.roots should reach into a submodule)", markers["ABC-1"])
	}

	// rules.roots explicitly targets the submodule's own git index and finds its markers.
	markers, missing, err = scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), []string{"sub"})
	if err != nil {
		t.Fatalf("scanRuleMarkers with rules.roots = %v, want nil", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (sub exists)", missing)
	}
	if len(markers["ABC-1"]) != 1 {
		t.Fatalf("markers[ABC-1] via rules.roots = %v, want 1 loc", markers["ABC-1"])
	}
}

func TestCheckLinksReportsScanError(t *testing.T) {
	root := unreadableTree(t)
	report := checkLinks(root)
	if len(report.findings) == 0 {
		t.Fatal("checkLinks = no findings, want error finding for unreadable subdirectory")
	}
	ok := false
	for _, f := range report.findings {
		if f.check == "links" && f.level == "error" && strings.Contains(f.message, "failed to scan") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("checkLinks findings %v do not include a scan error", report.findings)
	}
}
