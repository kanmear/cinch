package check

import (
	"os"
	"path/filepath"
	"testing"

	"cinch/internal/manifest"
)

// runCheckAudit runs C15 against a synthetic repo root. vars carries the
// manifest variables the checker reads (paths.audit, paths.domain,
// paths.tests.*).
func runCheckAudit(t *testing.T, files map[string]string, vars map[string]string) []finding {
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
	m := &manifest.Manifest{Vars: vars}
	r := &report{}
	checkAuditReport(root, m, r)
	return r.findings
}

// ruleDoc builds a one-rule domain doc (an audit case whose report covers a
// single rule needs a doc with exactly that rule — C15 also checks that every
// closure rule has a row).
func ruleDoc(id, text string) string {
	return "---\nrule_prefix: SIG\n---\n\n" + "1. **" + id + "** " + text + "\n"
}

const (
	sig001 = "Editing a tab is always free — never locked by a signature request."
	sig002 = "At most one pending signature request can exist per tab at a time."
	sig003 = "Creating a signature request requires `edit` permission on the tab's category."
	sig004 = "A display name is derived from another field."
)

const auditTest = "func TestEditTab_AlwaysFree(t *testing.T) {\n" +
	"\tassert.Equal(t, http.StatusOK, rec.Code)\n" +
	"}\n"

func auditVarsMap() map[string]string {
	return map[string]string{
		"paths.audit":             ".agent/audit/coverage.md",
		"paths.domain":            ".agent/domain",
		"paths.tests.integration": "backend/tests",
	}
}

func auditReport(body string) string {
	return "# Rule Coverage Audit Report\n\n" +
		"## signatures.md\n\n" +
		"| Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim) |\n" +
		"|---|---|---|---|---|---|---|\n" +
		body
}

func TestCheckAudit(t *testing.T) {
	base := map[string]string{
		"backend/tests/tab_test.go": auditTest,
	}
	doc := func(id, text string) map[string]string {
		return merge(base, map[string]string{".agent/domain/signatures.md": ruleDoc(id, text)})
	}
	row := func(id, text, status, file, name, assert string) string {
		return "| " + id + " | `" + text + "` | Yes | " + status + " | `" + file + "` | `" + name + "` | `" + assert + "` |\n"
	}

	t.Run("green covered row", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("vacuous row errors", func(t *testing.T) {
		// The recorded pi failure: no rule text, no test name, ✅.
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-001 | | Yes | ✅ | `tab_test.go` | handler test | |\n"),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "quotes no rule text")
		hasFinding(t, fs, "C15", "error", "does not appear in")
		hasFinding(t, fs, "C15", "error", "quotes no assertion")
	})

	t.Run("paraphrased rule text errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", "Editing is always free", "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "not verbatim")
	})

	t.Run("fabricated test name errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "✅", "tab_test.go", "TestCreateCategory", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "does not appear in")
	})

	t.Run("fabricated assertion errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert the response is a 409 conflict")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "not verbatim")
	})

	t.Run("unknown test file errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "✅", "handlers/ghost_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "does not exist")
	})

	t.Run("unknown status errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "POSSIBLY", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "unknown status")
	})

	t.Run("unresolvable rule errors", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-099", "No such rule.", "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "names no rule")
	})

	t.Run("missing row needs only the rule text", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-002", sig002), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-002 | `" + sig002 + "` | Yes | ⚠️ MISSING | — | — | — |\n"),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("N/A row needs only the rule text", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-004", sig004), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-004 | `" + sig004 + "` | N/A | N/A | — | — | — |\n"),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("tension row quotes the contradicting assertion", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-002", sig002), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-002", sig002, "⚠️ TENSION", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("multi-line rule body matches a fragment", func(t *testing.T) {
		doc := "---\nrule_prefix: SIG\n---\n\n" +
			"1. **SIG-003** " + sig003 + "\n" +
			"   A continuation line carries the rest of the rule.\n"
		fs := runCheckAudit(t, merge(base, map[string]string{
			".agent/domain/signatures.md": doc,
			".agent/audit/coverage.md": auditReport(
				"| SIG-003 | `A continuation line carries the rest of the rule.` | Yes | ⚠️ MISSING | — | — | — |\n"),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("backticked rule text matches", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-003", sig003), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-003 | `Creating a signature request requires `edit` permission on the tab's category.` | Yes | ⚠️ MISSING | — | — | — |\n"),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("markdown-escaped backticks match", func(t *testing.T) {
		// A model wraps the cell in backticks and escapes the interior
		// backticks as \` — the markdown form of a backtick in a code span.
		fs := runCheckAudit(t, merge(doc("SIG-003", sig003), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-003 | `Creating a signature request requires \\`edit\\` permission on the tab's category.` | Yes | ⚠️ MISSING | — | — | — |\n"),
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("test file resolves under paths.tests", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(row("SIG-001", sig001, "✅", "handlers/tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
			"backend/tests/handlers/tab_test.go": auditTest,
		}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("undeclared audit path skips", func(t *testing.T) {
		r := &report{}
		checkAuditReport(t.TempDir(), &manifest.Manifest{Vars: map[string]string{}}, r)
		if len(r.findings) != 0 {
			t.Fatalf("expected skip, got %v", r.findings)
		}
	})

	t.Run("rows outside domain sections are ignored", func(t *testing.T) {
		report := "# Rule Coverage Audit Report\n\n" +
			"## N/A Declarations\n\n" +
			"| Rule | Domain |\n" +
			"|---|---|\n" +
			"| `SIG-004` | signatures.md |\n\n" +
			"## Summary\n\n" +
			"| Domain | Total | Covered |\n" +
			"|---|---|---|\n" +
			"| SIG | 4 | 3/3 |\n" +
			auditReport(row("SIG-001", sig001, "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)"))
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{".agent/audit/coverage.md": report}), auditVarsMap())
		if len(fs) != 0 {
			t.Fatalf("expected green, got %v", fs)
		}
	})

	t.Run("malformed row errors on columns", func(t *testing.T) {
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": auditReport(
				"| SIG-001 | only a rule text | ✅ |\n"),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "7 canonical columns")
	})

	t.Run("missing rows error", func(t *testing.T) {
		fullDoc := "---\nrule_prefix: SIG\n---\n\n" +
			"1. **SIG-001** " + sig001 + "\n" +
			"2. **SIG-002** " + sig002 + "\n" +
			"3. **SIG-003** " + sig003 + "\n" +
			"4. **SIG-004** " + sig004 + "\n"
		fs := runCheckAudit(t, merge(base, map[string]string{
			".agent/domain/signatures.md": fullDoc,
			".agent/audit/coverage.md":    auditReport(row("SIG-001", sig001, "✅", "tab_test.go", "TestEditTab_AlwaysFree", "assert.Equal(t, http.StatusOK, rec.Code)")),
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "missing coverage rows")
		hasFinding(t, fs, "C15", "error", "SIG-002, SIG-003, SIG-004")
	})

	t.Run("empty report errors", func(t *testing.T) {
		// The pi probe's format-deviant report: no canonical rows at all.
		fs := runCheckAudit(t, merge(doc("SIG-001", sig001), map[string]string{
			".agent/audit/coverage.md": "# Rule Coverage Audit Report\n\n## Summary\n\nNo per-domain tables.\n",
		}), auditVarsMap())
		hasFinding(t, fs, "C15", "error", "has no coverage rows")
	})
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
