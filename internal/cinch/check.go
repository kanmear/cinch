package cinch

import (
	"fmt"
	"path/filepath"
	"sort"

	"cinch/internal/output"
)

const pathsDocsKey = "paths.docs"
const defaultDocsPath = ".docs"

// docsPathValue returns m's paths.docs value, or the default when unset or m
// is nil. Shared with render.go so {{paths.docs}} in templates always
// resolves to the same root cinch check itself scans.
func docsPathValue(m *Manifest) string {
	if m != nil {
		if v, ok := m.Vars[pathsDocsKey]; ok && v != "" {
			return v
		}
	}
	return defaultDocsPath
}

// ResolveDocsRoot determines the docs corpus location for root, honoring an
// optional `paths.docs` key in cinch.yml. Defaults to ".docs" when no
// manifest, or no such key, is present — check must stay zero-config by
// default. The value is always resolved against root: Manifest.validate
// guarantees it is repo-local, which is what keeps this resolution and
// render's agreeing on one directory.
func ResolveDocsRoot(root string) (string, error) {
	m, err := loadManifestOptional(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, docsPathValue(m)), nil
}

// Finding is one thing a check found wrong with the tree.
type Finding struct {
	Check   string // "links" | "rules" | "coupling" | "generated" | "commit" | "core"
	Level   string // "error" | "block"
	File    string // repo-relative
	Line    int    // 1-based
	Message string
}

// CmdCheck runs every check against the current working directory and
// prints findings to stdout, one per line. msgFile, if non-empty, is a path
// to a file containing the in-progress commit message (wired via a
// commit-msg git hook) — only the coupling check's rule-reword escape hatch
// consults it.
func CmdCheck(msgFile string) int {
	docsRoot, err := ResolveDocsRoot(".")
	if err != nil {
		return output.Fail("check", err)
	}

	var findings []Finding

	linksFindings := checkLinks(docsRoot)
	findings = append(findings, linksFindings...)
	output.CheckStatus("links", len(linksFindings), "")

	rulesFindings := checkRules(docsRoot, ".")
	findings = append(findings, rulesFindings...)
	output.CheckStatus("rules", len(rulesFindings), "")

	coupling := checkCoupling(docsRoot, ".", msgFile)
	findings = append(findings, coupling.Findings...)
	output.CheckStatus("coupling", len(coupling.Findings), coupling.NoOp)
	for _, s := range coupling.Suppressed {
		output.Skip("check", "coupling", s)
	}

	generated := checkGenerated(".")
	findings = append(findings, generated.Findings...)
	output.CheckStatus("generated", len(generated.Findings), generated.NoOp)

	commit := checkCommit(".", msgFile)
	findings = append(findings, commit.Findings...)
	output.CheckStatus("commit", len(commit.Findings), commit.NoOp)

	pin := checkPin(".", Version)
	findings = append(findings, pin.Findings...)
	output.CheckStatus("core", len(pin.Findings), pin.NoOp)

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})

	for _, f := range findings {
		fmt.Printf("%s %s %s:%d: %s\n", f.Check, output.Level(f.Level), f.File, f.Line, f.Message)
	}

	if len(findings) > 0 {
		return 1
	}
	return 0
}
