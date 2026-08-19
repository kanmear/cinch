package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseRuleItemsStr(t *testing.T, body string) []ruleItem {
	t.Helper()
	return parseRuleItems("test.md", []byte(body))
}

func TestParseRuleItemsBasic(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** first rule\n2. **DEF-2** second rule\n")
	want := []ruleItem{
		{ID: "ABC-1", File: "test.md", Line: 1, Text: "1. **ABC-1** first rule"},
		{ID: "DEF-2", File: "test.md", Line: 2, Text: "2. **DEF-2** second rule"},
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
	if got[0].Text != wantText {
		t.Fatalf("Text = %q, want %q", got[0].Text, wantText)
	}
}

func TestParseRuleItemsSkipsFencedCode(t *testing.T) {
	body := "1. **ABC-1** visible\n```\n1. **FAKE-9** hidden\n```\n"
	got := parseRuleItemsStr(t, body)
	if len(got) != 1 || got[0].ID != "ABC-1" {
		t.Fatalf("parseRuleItems = %v, want only ABC-1", got)
	}
}

func TestParseRuleItemsCRLF(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** rule\r\n2. **DEF-2** next\r\n")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Line != 1 || got[1].Line != 2 {
		t.Fatalf("line numbers = %d,%d, want 1,2", got[0].Line, got[1].Line)
	}
	if got[0].Text != "1. **ABC-1** rule" {
		t.Fatalf("Text = %q, want no carriage returns", got[0].Text)
	}
}

func TestParseRuleItemsIgnore(t *testing.T) {
	t.Run("no reason", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n<!-- cinch:ignore -->\n")
		if len(got) != 1 || !got[0].HasIgnore || got[0].IgnoreReason != "" || got[0].IgnoreLine != 2 {
			t.Fatalf("item = %+v, want HasIgnore with empty reason at line 2", got[0])
		}
	})

	t.Run("with reason", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n<!-- cinch:ignore: historical debt -->\n")
		if len(got) != 1 || !got[0].HasIgnore || got[0].IgnoreReason != "historical debt" {
			t.Fatalf("item = %+v, want HasIgnore with reason", got[0])
		}
	})

	t.Run("inside fence ignored", func(t *testing.T) {
		got := parseRuleItemsStr(t, "1. **ABC-1** rule\n```\n<!-- cinch:ignore -->\n```\n")
		if len(got) != 1 || got[0].HasIgnore {
			t.Fatalf("item = %+v, want HasIgnore false", got[0])
		}
	})
}

func TestParseRuleItemsPlainNumberedItemFlushes(t *testing.T) {
	got := parseRuleItemsStr(t, "1. **ABC-1** rule\n2. plain item\n3. **DEF-2** next\n")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "ABC-1" || got[0].Text != "1. **ABC-1** rule" {
		t.Fatalf("first item = %+v, want ABC-1 without trailing plain item text", got[0])
	}
	if got[1].ID != "DEF-2" {
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
	root := unreadableTree(t)
	findings := checkRules(root, ".")
	if len(findings) == 0 {
		t.Fatal("checkRules = no findings, want error finding for unreadable subdirectory")
	}
	ok := false
	for _, f := range findings {
		if f.Check == "rules" && f.Level == "error" && strings.Contains(f.Message, "failed to scan") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("checkRules findings %+v do not include a scan error", findings)
	}
}

func TestCheckRulesScanErrorDoesNotReportUnmarked(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "rules.md"), []byte("1. **ABC-1** rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A broken symlink makes the marker scan fail after the docs scan
	// succeeds, isolating the marker-scan error path.
	if err := os.Symlink(filepath.Join(root, "missing.go"), filepath.Join(root, "broken.go")); err != nil {
		t.Fatal(err)
	}

	findings := checkRules(docs, root)
	scanErr := false
	unmarked := 0
	for _, f := range findings {
		if f.Check == "rules" && strings.Contains(f.Message, "failed to scan") {
			scanErr = true
		}
		if f.Check == "rules" && strings.Contains(f.Message, "no // cinch:rule marker") {
			unmarked++
		}
	}
	if !scanErr {
		t.Fatalf("checkRules findings %+v do not include a scan error", findings)
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

func TestScanRuleMarkersGitScope(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append([]string{
			"HOME=" + root,
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
		}, os.Environ()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
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

func TestCheckLinksReportsScanError(t *testing.T) {
	root := unreadableTree(t)
	findings := checkLinks(root)
	if len(findings) == 0 {
		t.Fatal("checkLinks = no findings, want error finding for unreadable subdirectory")
	}
	ok := false
	for _, f := range findings {
		if f.Check == "links" && f.Level == "error" && strings.Contains(f.Message, "failed to scan") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("checkLinks findings %v do not include a scan error", findings)
	}
}
