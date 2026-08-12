package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
// default. An absolute value is used as-is; a relative value is resolved
// against root.
func ResolveDocsRoot(root string) (string, error) {
	m, err := loadManifestOptional(root)
	if err != nil {
		return "", err
	}
	val := docsPathValue(m)
	if filepath.IsAbs(val) {
		return val, nil
	}
	return filepath.Join(root, val), nil
}

// Finding is one thing a check found wrong with the tree.
type Finding struct {
	Check   string // "links" | "rules" | "coupling" | "generated"
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
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}

	var findings []Finding
	findings = append(findings, checkLinks(docsRoot)...)
	findings = append(findings, checkRules(docsRoot, ".")...)

	coupling := checkCoupling(docsRoot, ".", msgFile)
	findings = append(findings, coupling.Findings...)
	if coupling.NoOp != "" {
		fmt.Fprintln(os.Stderr, coupling.NoOp)
	}
	for _, s := range coupling.Suppressed {
		fmt.Fprintln(os.Stderr, "coupling: "+s)
	}

	generated := checkGenerated(".")
	findings = append(findings, generated.Findings...)
	if generated.NoOp != "" {
		fmt.Fprintln(os.Stderr, generated.NoOp)
	}

	findings = append(findings, checkCommit(".", msgFile)...)

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
		fmt.Printf("%s %s %s:%d: %s\n", f.Check, f.Level, f.File, f.Line, f.Message)
	}

	if len(findings) > 0 {
		return 1
	}
	return 0
}
