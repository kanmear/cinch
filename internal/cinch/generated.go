package cinch

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// generatedResult separates real findings from the no-op case: no manifest,
// or no rendered output yet, is not a pass — it is nothing to verify, and
// must say so rather than read as clean.
type generatedResult struct {
	Findings []Finding
	NoOp     string
}

// checkGenerated compares the render output on disk against what `cinch
// render` would produce right now, byte for byte — this is the check that
// makes the README's "a re-render diff proves tampering" true. A missing or
// altered generated file is a finding; so is a leftover generated file that
// the current render no longer produces (an orphan).
func checkGenerated(root string) generatedResult {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return generatedResult{NoOp: "generated: cinch render has not run — nothing to verify"}
	}

	docsRoot := docsPathValue(m)
	workflowsDir := filepath.Join(root, docsRoot, "workflows")
	if info, err := os.Stat(workflowsDir); err != nil || !info.IsDir() {
		return generatedResult{NoOp: "generated: cinch render has not run — nothing to verify"}
	}

	files, err := renderAll(m, root)
	if err != nil {
		return generatedResult{NoOp: "generated: " + err.Error() + " — cannot verify, cinch render would also fail"}
	}

	expected := map[string]bool{}
	renderDirs := map[string]bool{}
	var findings []Finding
	for _, f := range files {
		dst := filepath.Join(root, f.Dest)
		expected[dst] = true
		renderDirs[filepath.Dir(dst)] = true

		want := header(f.Source, f.Body) + f.Body
		got, err := os.ReadFile(dst)
		switch {
		case err != nil:
			findings = append(findings, Finding{
				Check: "generated", Level: "error", File: f.Dest, Line: 1,
				Message: "missing — run `cinch render` (or revert the edit)",
			})
		case string(got) != want:
			findings = append(findings, Finding{
				Check: "generated", Level: "error", File: f.Dest, Line: 1,
				Message: "does not match a fresh render — run `cinch render` (or revert the edit)",
			})
		}
	}

	for dir := range renderDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if expected[p] {
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil || !strings.HasPrefix(string(data), headerPrefix) {
				continue
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				rel = p
			}
			findings = append(findings, Finding{
				Check: "generated", Level: "error", File: rel, Line: 1,
				Message: "orphaned generated file, no longer produced by cinch render — delete it (cinch render never removes files)",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	return generatedResult{Findings: findings}
}
