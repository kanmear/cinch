package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulePrefix(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"present", "---\nrule_prefix: SIG\nowns:\n  - x\n---\n\n# D\n", "SIG"},
		{"absent", "---\nowns:\n  - x\n---\n\n# D\n", ""},
		{"no front matter", "# D\n\n1. rule\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rulePrefix(c.text); got != c.want {
				t.Fatalf("rulePrefix() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseDomainDoc(t *testing.T) {
	text := "---\nrule_prefix: SIG\n---\n\n# Signatures Business Rules\n\n## General\n\n" +
		"1. **SIG-001** Editing a tab is always free — never locked by a signature request.\n" +
		"2. **SIG-002** At most one pending signature request can exist per tab at a time.\n" +
		"   Multi-line continuation is not a new item.\n" +
		"3. A numbered item with no ID is pre-migration.\n" +
		"4. **WRONG-004** Prefix mismatch.\n\n" +
		"## Example (not rules)\n\n```\n1. **SIG-999** This is inside a fence and must be ignored.\n```\n\n" +
		"<!-- cinch:ignore SIG-002 -->\n"

	dd := parseDomainDoc(text)
	if dd.prefix != "SIG" {
		t.Fatalf("prefix = %q, want SIG", dd.prefix)
	}
	if len(dd.rules) != 3 {
		t.Fatalf("rules = %d, want 3 (fenced SIG-999 must be skipped): %v", len(dd.rules), dd.rules)
	}
	if dd.rules["SIG-001"] != "Editing a tab is always free — never locked by a signature request." {
		t.Fatalf("SIG-001 text = %q", dd.rules["SIG-001"])
	}
	if _, ok := dd.rules["SIG-999"]; ok {
		t.Fatal("fenced item was parsed as a rule")
	}
	if len(dd.noID) != 1 || dd.noID[0] != "A numbered item with no ID is pre-migration." {
		t.Fatalf("noID = %v", dd.noID)
	}
	if len(dd.ignores) != 1 || dd.ignores[0] != "SIG-002" {
		t.Fatalf("ignores = %v", dd.ignores)
	}
}

// mkMarker builds a marker line at runtime: the literal token must never
// appear in this repo's own source, or cinch's self-check would read its own
// test fixtures as stray markers (C8 reverse direction).
func mkMarker(id string) string { return "// cinch:r" + "ule " + id }

func TestScanRuleMarkers(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("backend/tests/handlers/tab_test.go", mkMarker("SIG-001")+"\nfunc TestX(t *testing.T) {}\n"+mkMarker("SIG-002")+" also inline")
	write("frontend/src/app.ts", mkMarker("SIG-003")+"\n")
	write(".git/config", mkMarker("GHOST")+"\n")
	write("bin/cinch", mkMarker("BIN")+"\n")
	write(".agent/conventions.md", mkMarker("DOC")+"\n")
	write("docs/ROADMAP.md", mkMarker("MAP")+"\n")
	write("templates/check-rules.md", mkMarker("TPL")+"\n")
	write("decisions.jsonl", mkMarker("LOG")+"\n")
	write("data.bin", "\x00\x01"+mkMarker("NUL"))

	got := scanRuleMarkers(root)
	for _, id := range []string{"SIG-001", "SIG-002", "SIG-003"} {
		if !got[id] {
			t.Fatalf("marker %s not found", id)
		}
	}
	for _, id := range []string{"GHOST", "BIN", "NUL", "DOC", "MAP", "LOG", "TPL"} {
		if got[id] {
			t.Fatalf("marker %s must be skipped (artifact/doc surface)", id)
		}
	}
}

// TestRuleIDsUnique — R5: rule IDs are permanent and never reused (D027). A
// duplicate ID across domain docs would corrupt the marker closure (a marker
// would resolve to two rules), so uniqueness is the machine-enforceable core
// of the permanence rule.
// cinch:rule HARNESS-005
func TestRuleIDsUnique(t *testing.T) {
	seen := map[string]string{}
	entries, err := os.ReadDir(".agent/domain")
	if err != nil {
		t.Fatalf("reading .agent/domain: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "overview.md" {
			continue
		}
		dd := parseDomainDoc(readFile(filepath.Join(".agent/domain", e.Name())))
		for id := range dd.rules {
			if prev, dup := seen[id]; dup {
				t.Errorf("rule ID %s appears in both %s and %s — IDs are never reused (R5)", id, prev, e.Name())
			}
			seen[id] = e.Name()
		}
	}
}

// checkRules findings helper: runs C8 against a synthetic repo root.
func runCheckRules(t *testing.T, files map[string]string) []finding {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := &Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
	r := &report{}
	checkRules(root, m, r)
	return r.findings
}

func hasFinding(t *testing.T, fs []finding, code, level, substr string) {
	t.Helper()
	for _, f := range fs {
		if f.code == code && f.level == level && strings.Contains(f.msg, substr) {
			return
		}
	}
	t.Fatalf("no %s/%s finding containing %q in: %v", code, level, substr, fs)
}

func TestCheckRules(t *testing.T) {

	doc := "---\nrule_prefix: SIG\n---\n\n1. **SIG-001** Rule one.\n2. **SIG-002** Rule two.\n"

	t.Run("green", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc,
			"backend/tests/tab_test.go":   mkMarker("SIG-001")+"\n"+mkMarker("SIG-002")+"\n",
		})
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("uncovered rule warns", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc,
			"backend/tests/tab_test.go":   mkMarker("SIG-001")+"\n",
		})
		hasFinding(t, fs, "C8", "warn", "SIG-002")
		if len(fs) != 1 {
			t.Fatalf("want exactly one finding, got %v", fs)
		}
	})

	t.Run("ignored rule is silent", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc + "\n<!-- cinch:ignore SIG-002 -->\n",
			"backend/tests/tab_test.go":   mkMarker("SIG-001")+"\n",
		})
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("stray marker errors", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc,
			"backend/tests/tab_test.go":   mkMarker("SIG-001")+"\n"+mkMarker("GHOST-001")+"\n",
		})
		hasFinding(t, fs, "C8", "error", "GHOST-001")
	})

	t.Run("marked and ignored is a contradiction", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc + "\n<!-- cinch:ignore SIG-001 -->\n",
			"backend/tests/tab_test.go":   mkMarker("SIG-001")+"\n",
		})
		hasFinding(t, fs, "C8", "error", "both marked and cinch:ignore'd")
	})

	t.Run("ignore for unknown rule errors", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": doc + "\n<!-- cinch:ignore SIG-999 -->\n",
		})
		hasFinding(t, fs, "C8", "error", "SIG-999")
	})

	t.Run("missing rule_prefix errors", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": "# D\n\n1. **SIG-001** Rule one.\n",
		})
		hasFinding(t, fs, "C8", "error", "no rule_prefix")
	})

	t.Run("prefix mismatch errors", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": "---\nrule_prefix: SIG\n---\n\n1. **TAB-001** Rule one.\n",
		})
		hasFinding(t, fs, "C8", "error", "TAB-001")
	})

	t.Run("item without ID errors", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/signatures.md": "---\nrule_prefix: SIG\n---\n\n1. Unnumbered rule text.\n",
		})
		hasFinding(t, fs, "C8", "error", "no rule ID")
	})

	t.Run("overview is exempt", func(t *testing.T) {
		fs := runCheckRules(t, map[string]string{
			".agent/domain/overview.md": "## R001 — The guardrail\nPhilosophies in the overview get no IDs (D027).\n",
		})
		if len(fs) != 0 {
			t.Fatalf("overview must be exempt, got %v", fs)
		}
	})

	t.Run("no domain layer skips", func(t *testing.T) {
		r := &report{}
		checkRules(t.TempDir(), &Manifest{Vars: map[string]string{}}, r)
		if len(r.findings) != 0 {
			t.Fatalf("expected skip, got %v", r.findings)
		}
	})
}
