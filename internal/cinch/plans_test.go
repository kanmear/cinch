package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const plansEnforceManifest = "paths:\n  docs: .docs\nplans:\n  lifecycle: enforce\n"

// plansTestRepo writes files, keyed by docs-relative path, under a fresh
// docs root next to a cinch.yml holding manifestBody.
func plansTestRepo(t *testing.T, manifestBody string, files map[string]string) (root, docsRoot string) {
	t.Helper()
	root = t.TempDir()
	docsRoot = filepath.Join(root, ".docs")
	writeTestFile(t, root, "cinch.yml", manifestBody)
	for name, content := range files {
		path := filepath.Join(docsRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Dir(path), filepath.Base(path), content)
	}
	return root, docsRoot
}

func checkPlansTest(t *testing.T, manifestBody string, files map[string]string) (checkResult, string) {
	t.Helper()
	root, docsRoot := plansTestRepo(t, manifestBody, files)
	return checkPlans(workingTreeRoots(root), docsRoot, loadTestManifest(t, root)), docsRoot
}

// cinch:rule CINCH-017
func TestParseStatusLine(t *testing.T) {
	filler := strings.Repeat("filler\n", 14)
	cases := []struct {
		name      string
		content   string
		wantWord  string
		wantLine  int
		wantFound bool
	}{
		{"word only", "# T\n\n**Status:** planned\n", "planned", 3, true},
		{"word then text", "**Status:** complete — shipped\n", "complete", 1, true},
		{"nothing after the marker", "**Status:**\n", "", 1, true},
		{"marker with trailing space only", "**Status:** \n", "", 1, true},
		{"no trailing newline", "**Status:** cut", "cut", 1, true},
		{"line 15 is read", filler + "**Status:** planned\n", "planned", 15, true},
		{"line 16 is not read", filler + "x\n**Status:** planned\n", "", 0, false},
		{"first status line wins", "**Status:** planned\n**Status:** complete\n", "planned", 1, true},
		{"indented marker is not a status line", "  **Status:** planned\n", "", 0, false},
		{"no bold is not a status line", "Status: planned\n", "", 0, false},
		{"empty content", "", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			word, line, found := parseStatusLine(c.content)
			if word != c.wantWord || line != c.wantLine || found != c.wantFound {
				t.Fatalf("parseStatusLine = (%q, %d, %v), want (%q, %d, %v)", word, line, found, c.wantWord, c.wantLine, c.wantFound)
			}
		})
	}
}

// cinch:rule CINCH-017
func TestCheckPlansKeyUnsetEnforcesNothing(t *testing.T) {
	result, _ := checkPlansTest(t, "paths:\n  docs: .docs\n", map[string]string{
		"plans/a.md": "# A\n\nno status here\n",
	})
	if len(result.findings) != 0 {
		t.Fatalf("findings = %+v, want none without plans.lifecycle", result.findings)
	}
	if want := "plans.lifecycle not set"; !strings.Contains(result.noOp, want) {
		t.Fatalf("noOp = %q, want it to say %q", result.noOp, want)
	}
}

// cinch:rule CINCH-017
func TestCheckPlansLiveFolderRejectsComplete(t *testing.T) {
	result, docsRoot := checkPlansTest(t, plansEnforceManifest, map[string]string{
		"plans/a.md": "# A\n\n**Status:** complete\n",
	})
	if len(result.findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", result.findings)
	}
	f := result.findings[0]
	if f.check != "plans" || f.level != "error" || f.file != filepath.Join(docsRoot, "plans", "a.md") || f.line != 3 {
		t.Fatalf("finding = %+v, want a plans error on plans/a.md line 3", f)
	}
	for _, want := range []string{`"complete"`, "planned", "in-progress"} {
		if !strings.Contains(f.message, want) {
			t.Fatalf("message = %q, want it to contain %q", f.message, want)
		}
	}
}

// cinch:rule CINCH-017
func TestCheckPlansArchiveRejectsInProgress(t *testing.T) {
	result, docsRoot := checkPlansTest(t, plansEnforceManifest, map[string]string{
		"plans/archive/a.md": "# A\n\n**Status:** in-progress\n",
	})
	if len(result.findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", result.findings)
	}
	f := result.findings[0]
	if f.file != filepath.Join(docsRoot, "plans", "archive", "a.md") || f.line != 3 {
		t.Fatalf("finding = %+v, want plans/archive/a.md line 3", f)
	}
	for _, want := range []string{`"in-progress"`, "complete", "cut"} {
		if !strings.Contains(f.message, want) {
			t.Fatalf("message = %q, want it to contain %q", f.message, want)
		}
	}
}

// cinch:rule CINCH-017
func TestCheckPlansReadsOnlyTheFirstFifteenLines(t *testing.T) {
	late := strings.Repeat("filler\n", 15) + "**Status:** planned\n"
	taskStatus := "# A\n\nno plan status\n" + strings.Repeat("filler\n", 14) + "### T1\n**Status:** [x]\n"
	result, docsRoot := checkPlansTest(t, plansEnforceManifest, map[string]string{
		"plans/late.md":  late,
		"plans/tasks.md": taskStatus,
	})
	if len(result.findings) != 2 {
		t.Fatalf("findings = %+v, want one per file", result.findings)
	}
	for _, f := range result.findings {
		if f.line != 1 || !strings.Contains(f.message, "no **Status:** line in the first 15 lines") ||
			!strings.Contains(f.message, "planned") || !strings.Contains(f.message, "in-progress") {
			t.Fatalf("finding = %+v, want line 1 saying no status line in the first 15 and the allowed words", f)
		}
	}
	if result.findings[0].file != filepath.Join(docsRoot, "plans", "late.md") &&
		result.findings[1].file != filepath.Join(docsRoot, "plans", "late.md") {
		t.Fatalf("findings = %+v, want one for late.md", result.findings)
	}
}

// cinch:rule CINCH-017
func TestCheckPlansStatusWordMustMatchExactly(t *testing.T) {
	files := map[string]string{
		"plans/archive/no-bold.md":   "# A\n\nStatus: complete\n",
		"plans/archive/capital.md":   "# A\n\n**Status:** Complete\n",
		"plans/archive/synonym.md":   "# A\n\n**Status:** done\n",
		"plans/archive/comma.md":     "# A\n\n**Status:** complete, shipped\n",
		"plans/archive/dash.md":      "# A\n\n**Status:** complete — shipped in abc123\n",
		"plans/archive/paren.md":     "# A\n\n**Status:** complete (abc123)\n",
		"plans/archive/cut-bare.md":  "# A\n\n**Status:** cut\n",
		"plans/archive/cut-prose.md": "# A\n\n**Status:** cut — superseded by plan 9\n",
	}
	result, docsRoot := checkPlansTest(t, plansEnforceManifest, files)
	got := map[string]bool{}
	for _, f := range result.findings {
		got[filepath.Base(f.file)] = true
	}
	for _, name := range []string{"no-bold.md", "capital.md", "synonym.md", "comma.md"} {
		if !got[name] {
			t.Errorf("no finding for %s under %s, want one", name, docsRoot)
		}
	}
	for _, name := range []string{"dash.md", "paren.md", "cut-bare.md", "cut-prose.md"} {
		if got[name] {
			t.Errorf("finding for %s, want it to pass", name)
		}
	}
	if len(result.findings) != 4 {
		t.Fatalf("findings = %+v, want exactly four", result.findings)
	}
}

// cinch:rule CINCH-017
func TestCheckPlansFixFolderUsesLiveWords(t *testing.T) {
	result, docsRoot := checkPlansTest(t, plansEnforceManifest, map[string]string{
		"plans/fix/ok.md":  "# F\n\n**Status:** planned\n",
		"plans/fix/bad.md": "# F\n\n**Status:** complete\n",
	})
	if len(result.findings) != 1 || result.findings[0].file != filepath.Join(docsRoot, "plans", "fix", "bad.md") {
		t.Fatalf("findings = %+v, want exactly one, on plans/fix/bad.md", result.findings)
	}
}

// cinch:rule CINCH-017
func TestCheckPlansIgnoresOtherSubfolders(t *testing.T) {
	result, _ := checkPlansTest(t, plansEnforceManifest, map[string]string{
		"plans/notes/x.md":          "# Notes\n\nno status\n",
		"plans/archive/x/y.md":      "# Deep\n\nno status\n",
		"plans/fix/deeper/z.md":     "# Deep\n\nno status\n",
		"other/plans/a.md":          "# Elsewhere\n\nno status\n",
		"plans/archive/ok.md":       "# A\n\n**Status:** complete\n",
		"plans/notes-not-md.txt":    "no status\n",
		"plans/fix/not-markdown.go": "package x\n",
	})
	if len(result.findings) != 0 {
		t.Fatalf("findings = %+v, want none: only plans/, plans/fix/ and plans/archive/ are checked", result.findings)
	}
}

// cinch:rule CINCH-017
func TestCheckPlansUnknownKeyValue(t *testing.T) {
	root, docsRoot := plansTestRepo(t, "paths:\n  docs: .docs\nplans:\n  lifecycle: strict\n", map[string]string{
		"plans/a.md": "# A\n\nno status\n",
	})
	result := checkPlans(workingTreeRoots(root), docsRoot, loadTestManifest(t, root))
	if len(result.findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", result.findings)
	}
	f := result.findings[0]
	if f.check != "plans" || f.level != "error" || f.file != filepath.Join(root, manifestPath) || f.line != 1 {
		t.Fatalf("finding = %+v, want a plans error on cinch.yml line 1", f)
	}
	for _, want := range []string{"strict", "enforce"} {
		if !strings.Contains(f.message, want) {
			t.Fatalf("message = %q, want it to contain %q", f.message, want)
		}
	}
}
