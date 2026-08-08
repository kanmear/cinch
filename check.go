package main

import (
	"fmt"
	"os"
	"sort"
)

// docsRoot is the fixed, non-configurable corpus root (see commit 8b3c0ae:
// "corpus root is .docs — single docs tree, cinch consumes its own spec").
const docsRoot = ".docs"

// Finding is one thing a check found wrong with the tree.
type Finding struct {
	Check   string // "links" | "rules" | "coupling"
	Level   string // "error" | "block"
	File    string // repo-relative
	Line    int    // 1-based
	Message string
}

// cmdCheck runs every check against the current working directory and
// prints findings to stdout, one per line. msgFile, if non-empty, is a path
// to a file containing the in-progress commit message (wired via a
// commit-msg git hook) — only the coupling check's rule-reword escape hatch
// consults it.
func cmdCheck(msgFile string) int {
	var findings []Finding
	findings = append(findings, checkLinks(docsRoot)...)
	findings = append(findings, checkRules(docsRoot, ".")...)

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

func usageError(msg string) int {
	fmt.Fprintln(os.Stderr, "cinch: "+msg)
	return 2
}
