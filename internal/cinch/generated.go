package cinch

import (
	"os"
	"path/filepath"
	"sort"
)

func checkGenerated(root string) checkResult {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return checkResult{noOp: "cinch render has not run — nothing to verify"}
	}

	files, err := renderAll(m)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "generated", level: "error", file: manifestPath, line: 1,
			message: err.Error() + " — cinch render fails here, so no generated output can be verified",
		}}}
	}

	rendered := false
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(root, f.dest)); err == nil {
			rendered = true
			break
		}
	}
	if !rendered {
		return checkResult{noOp: "cinch render has not run — nothing to verify"}
	}

	expected := map[string]bool{}
	renderDirs := map[string]bool{}
	var findings []finding
	for _, f := range files {
		dst := filepath.Join(root, f.dest)
		expected[dst] = true
		renderDirs[filepath.Dir(dst)] = true

		want := header(f.source, f.body, f.style) + f.body
		got, err := os.ReadFile(dst)
		switch {
		case err != nil:
			findings = append(findings, finding{
				check: "generated", level: "error", file: f.dest, line: 1,
				message: "missing — run 'cinch render' (or revert the edit)",
			})
		case string(got) != want:
			findings = append(findings, finding{
				check: "generated", level: "error", file: f.dest, line: 1,
				message: "does not match a fresh render — run 'cinch render' (or revert the edit)",
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
			rel := relTo(root, p)
			findings = append(findings, finding{
				check: "generated", level: "error", file: rel, line: 1,
				message: "orphaned generated file, no longer produced by cinch render — delete it (cinch render never removes files)",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].file < findings[j].file })
	return checkResult{findings: findings}
}
