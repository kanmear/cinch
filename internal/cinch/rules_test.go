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
	if _, err := scanRuleMarkers(root, filepath.Join(root, "docs")); err == nil {
		t.Fatal("scanRuleMarkers = nil error, want error for unreadable file")
	}
}

func TestCheckRulesReportsScanError(t *testing.T) {
	docsRoot := unreadableTree(t)
	report := checkRules(".", docsRoot)
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

	report := checkRules(repoRoot, docsRoot)
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

	markers, err := scanRuleMarkers(root, filepath.Join(root, "docs"))
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

	markers, err := scanRuleMarkers(root, filepath.Join(root, "docs"))
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

	markers, err := scanRuleMarkers(root, filepath.Join(root, "docs"))
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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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

	report := checkRules(root, docsRoot)

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
