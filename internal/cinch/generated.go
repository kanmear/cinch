package cinch

import (
	"os"
	"path/filepath"
	"sort"
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
		return generatedResult{NoOp: "cinch render has not run — nothing to verify"}
	}

	// A render that would fail is a defect this check can decide, not an
	// absence: reporting it as a no-op means `cinch render` fails loudly
	// while `cinch check` — the thing wired into the hook — goes quiet, so
	// one untitled doc silently disables verification of every generated
	// file in the repo.
	files, err := renderAll(m)
	if err != nil {
		return generatedResult{Findings: []Finding{{
			Check: "generated", Level: "error", File: manifestPath, Line: 1,
			Message: err.Error() + " — cinch render fails here, so no generated output can be verified",
		}}}
	}

	// "render has not run" is the absence of every expected output, not the
	// absence of one directory. Gating on the workflows dir alone skipped
	// verification of the outputs that live outside the docs root — the hook
	// shims — so deleting that directory disabled the check that makes them
	// tamper-evident. Absence of everything stays a no-op, never a pass.
	rendered := false
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(root, f.Dest)); err == nil {
			rendered = true
			break
		}
	}
	if !rendered {
		return generatedResult{NoOp: "cinch render has not run — nothing to verify"}
	}

	expected := map[string]bool{}
	renderDirs := map[string]bool{}
	var findings []Finding
	for _, f := range files {
		dst := filepath.Join(root, f.Dest)
		expected[dst] = true
		renderDirs[filepath.Dir(dst)] = true

		want := header(f.Source, f.Body, f.Style) + f.Body
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

	// Also scan the docs/hooks roots implied by the last *committed* manifest,
	// not just this render pass's roots. A paths.docs edit that hasn't been
	// paired with moving the directory (the mistake `cinch move-docs` exists to
	// prevent) leaves the old directory's generated files completely outside
	// renderDirs — invisible to every check — for exactly as long as the stale
	// directory and the uncommitted manifest edit coexist, i.e. up to the
	// commit that should have moved it too. Bounded to one comparison against
	// HEAD, not a general history scan.
	if prevData, ok := gitShow(root, "HEAD:"+manifestPath); ok {
		if prevM, err := parseManifestBytes(prevData, manifestPath+"@HEAD"); err == nil {
			renderDirs[filepath.Join(root, docsPathValue(prevM), workflowsSubdir)] = true
			renderDirs[filepath.Join(root, hooksPathValue(prevM))] = true
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
			if err != nil || !hasGeneratedHeader(string(data)) {
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
