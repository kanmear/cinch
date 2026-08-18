package cinch

import (
	"os"
	"path/filepath"
	"sort"
)

type generatedResult struct {
	Findings []Finding
	NoOp     string
}

func checkGenerated(root string) generatedResult {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return generatedResult{NoOp: "cinch render has not run — nothing to verify"}
	}

	files, err := renderAll(m)
	if err != nil {
		return generatedResult{Findings: []Finding{{
			Check: "generated", Level: "error", File: manifestPath, Line: 1,
			Message: err.Error() + " — cinch render fails here, so no generated output can be verified",
		}}}
	}

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
