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
