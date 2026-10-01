package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

// impactHit names one (rule ID, changed file, reason) triple a diff
// plausibly touches.
type impactHit struct {
	ruleID string
	reason string // "owns" or "marker"
	file   string
}

// impactFor surfaces two independent match rules, both named on the hit
// rather than collapsed: an owns: match and a marker match.
func impactFor(changedFiles []string, docs []docFrontmatterJSON, markers map[string][]markerLoc, byPrefixRuleIDs func(prefix string) []string) []impactHit {
	var hits []impactHit
	seen := map[[3]string]bool{}
	add := func(ruleID, reason, file string) {
		key := [3]string{ruleID, reason, file}
		if seen[key] {
			return
		}
		seen[key] = true
		hits = append(hits, impactHit{ruleID: ruleID, reason: reason, file: file})
	}

	for _, doc := range docs {
		if doc.RulePrefix == "" {
			continue
		}
		for _, changed := range changedFiles {
			if !ownsMatches(changed, doc.Owns) {
				continue
			}
			for _, id := range byPrefixRuleIDs(doc.RulePrefix) {
				add(id, "owns", changed)
			}
		}
	}

	for id, locs := range markers {
		for _, loc := range locs {
			for _, changed := range changedFiles {
				if changed == loc.file {
					add(id, "marker", changed)
				}
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].ruleID != hits[j].ruleID {
			return hits[i].ruleID < hits[j].ruleID
		}
		if hits[i].reason != hits[j].reason {
			return hits[i].reason < hits[j].reason
		}
		return hits[i].file < hits[j].file
	})
	return hits
}

// ownsMatches is prefix-based, not true globbing — matches the existing
// hookWhenMatches convention (hook.go). See warnGlobLikeOwns for the nudge
// this implies for an owns: entry containing glob syntax.
func ownsMatches(changed string, owns []string) bool {
	for _, entry := range owns {
		if changed == entry || strings.HasPrefix(changed, entry+"/") {
			return true
		}
	}
	return false
}

func buildImpact(repoRoot, docsRoot string, changedFiles []string) ([]impactHit, error) {
	docs, err := scanDocFrontmatter(docsRoot)
	if err != nil {
		return nil, err
	}
	// extraRoots (rules.roots) is deliberately not threaded in here: impact
	// only ever matches a marker's file against changedFiles, which are
	// always repoRoot-relative (staged paths or CLI args) — a sibling root's
	// files can never appear there, so scanning them would be pure overhead.
	rawMarkers, _, err := scanRuleMarkers(repoRoot, docsRoot, nil)
	if err != nil {
		return nil, err
	}
	// scanRuleMarkers's file locations are joined against repoRoot; changed
	// files (from git or from CLI args) are always repoRoot-relative, so
	// normalize before matching. A no-op when repoRoot is "." (every real
	// caller here — CmdImpact and the pre-commit hook both pass root=".").
	markers := make(map[string][]markerLoc, len(rawMarkers))
	for id, locs := range rawMarkers {
		normalized := make([]markerLoc, len(locs))
		for i, loc := range locs {
			normalized[i] = markerLoc{file: filepath.ToSlash(mdscan.RelTo(repoRoot, loc.file)), line: loc.line}
		}
		markers[id] = normalized
	}
	items, err := scanRuleDocs(docsRoot)
	if err != nil {
		return nil, err
	}

	byPrefix := func(prefix string) []string {
		var ids []string
		for _, it := range items {
			if strings.HasPrefix(it.id, prefix) {
				ids = append(ids, it.id)
			}
		}
		return ids
	}

	return impactFor(changedFiles, docs, markers, byPrefix), nil
}

// printImpact is the advisory-only output path: never a finding, never
// affects an exit code. Prints nothing when there's nothing to report.
func printImpact(hits []impactHit) {
	if len(hits) == 0 {
		return
	}
	var order []string
	byRule := map[string][]impactHit{}
	for _, h := range hits {
		if _, ok := byRule[h.ruleID]; !ok {
			order = append(order, h.ruleID)
		}
		byRule[h.ruleID] = append(byRule[h.ruleID], h)
	}
	fmt.Fprintln(os.Stderr, "cinch impact: this diff plausibly touches:")
	for _, id := range order {
		var files []string
		for _, h := range byRule[id] {
			files = append(files, h.file+" ("+h.reason+")")
		}
		fmt.Fprintf(os.Stderr, "  %s — %s — confirm the docs still match\n", id, strings.Join(files, ", "))
	}
}

// CmdImpact is a standalone entry point (cinch impact [FILES...]); when no
// files are given it defaults to the staged diff. Unlike the pre-commit
// hook's use of buildImpact, a scan error here surfaces loudly — the user
// asked for this data directly and didn't get it.
func CmdImpact(root string, files []string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("impact", err)
	}
	if len(files) == 0 {
		files, err = stagedPaths(root)
		if err != nil {
			return output.Fail("impact", err)
		}
	}
	hits, err := buildImpact(root, docsRoot, files)
	if err != nil {
		return output.Fail("impact", err)
	}
	printImpact(hits)
	return 0
}
