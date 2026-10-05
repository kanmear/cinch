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

// impactHit is one thing a diff plausibly touches, of one of two kinds. A
// marker hit names a single rule and the marker locations that sit in
// changed files. An owns hit names a doc, the changed files under its
// owns: paths, and every rule that doc declares — one hit per doc, not one
// per rule.
type impactHit struct {
	kind string // "marker" or "owns"

	// marker hits
	ruleID  string
	markers []markerLoc

	// owns hits
	doc     string
	files   []string
	ruleIDs []string
}

// impactFor surfaces two independent match rules, both named on the hit
// rather than collapsed: a marker match (per rule) and an owns: match (per
// doc, attributing the rules declared in that doc's file). Marker hits come
// first, sorted by rule ID, then owns hits sorted by doc path.
func impactFor(changedFiles []string, docs []docFrontmatterJSON, markers map[string][]markerLoc, rulesByDoc map[string][]string) []impactHit {
	var marked, owned []impactHit

	for id, locs := range markers {
		var inDiff []markerLoc
		for _, loc := range locs {
			for _, changed := range changedFiles {
				if changed == loc.file {
					inDiff = append(inDiff, loc)
					break
				}
			}
		}
		if len(inDiff) == 0 {
			continue
		}
		sort.Slice(inDiff, func(i, j int) bool {
			if inDiff[i].file != inDiff[j].file {
				return inDiff[i].file < inDiff[j].file
			}
			return inDiff[i].line < inDiff[j].line
		})
		marked = append(marked, impactHit{kind: "marker", ruleID: id, markers: inDiff})
	}
	sort.Slice(marked, func(i, j int) bool { return marked[i].ruleID < marked[j].ruleID })

	seenDoc := map[string]bool{}
	for _, doc := range docs {
		ids := rulesByDoc[doc.File]
		if len(ids) == 0 || seenDoc[doc.File] {
			continue
		}
		seenDoc[doc.File] = true
		var files []string
		seenFile := map[string]bool{}
		for _, changed := range changedFiles {
			if !seenFile[changed] && ownsMatches(changed, doc.Owns) {
				seenFile[changed] = true
				files = append(files, changed)
			}
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		owned = append(owned, impactHit{kind: "owns", doc: doc.File, files: files, ruleIDs: ids})
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].doc < owned[j].doc })

	return append(marked, owned...)
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

// buildImpact reads content from roots.fsRoot, so docsRoot must sit under it:
// the hook passes an index snapshot and CmdImpact the working tree.
func buildImpact(roots checkRoots, docsRoot string, changedFiles, exclude []string) ([]impactHit, error) {
	docs, err := scanDocFrontmatter(docsRoot)
	if err != nil {
		return nil, err
	}
	// extraRoots (rules.roots) is deliberately not threaded in here: impact
	// only ever matches a marker's file against changedFiles, which are
	// always repo-relative (staged paths or CLI args) — a sibling root's
	// files can never appear there, so scanning them would be pure overhead.
	rawMarkers, _, err := scanRuleMarkers(roots, docsRoot, markerScanOptions{exclude: exclude})
	if err != nil {
		return nil, err
	}
	// scanRuleMarkers's file locations are joined against fsRoot; changed
	// files (from git or from CLI args) are always repo-relative, so
	// normalize before matching. This also keeps a snapshot's temporary
	// directory out of the output. A no-op when fsRoot is "." (CmdImpact and
	// the pre-commit hook's fast path both pass root=".").
	markers := make(map[string][]markerLoc, len(rawMarkers))
	for id, locs := range rawMarkers {
		normalized := make([]markerLoc, len(locs))
		for i, loc := range locs {
			normalized[i] = markerLoc{file: filepath.ToSlash(mdscan.RelTo(roots.fsRoot, loc.file)), line: loc.line}
		}
		markers[id] = normalized
	}
	items, err := scanRuleDocs(docsRoot)
	if err != nil {
		return nil, err
	}

	rulesByDoc := map[string][]string{}
	seen := map[[2]string]bool{}
	for _, it := range items {
		if seen[[2]string{it.file, it.id}] {
			continue
		}
		seen[[2]string{it.file, it.id}] = true
		rulesByDoc[it.file] = append(rulesByDoc[it.file], it.id)
	}

	hits := impactFor(changedFiles, docs, markers, rulesByDoc)
	for i := range hits {
		if hits[i].kind == "owns" {
			hits[i].doc = filepath.ToSlash(mdscan.RelTo(roots.fsRoot, hits[i].doc))
		}
	}
	return hits, nil
}

// maxImpactListed caps how many changed files and rule IDs one owns line
// names before collapsing the rest into "+N more".
const maxImpactListed = 3

func capList(items []string) string {
	if len(items) <= maxImpactListed {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(items[:maxImpactListed], ", "), len(items)-maxImpactListed)
}

// printImpact is the advisory-only output path: never a finding, never
// affects an exit code. Prints nothing when there's nothing to report.
// Marker lines come first, then one owns line per doc, then a single
// closing reminder.
func printImpact(hits []impactHit) {
	if len(hits) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "cinch impact: this diff plausibly touches:")
	for _, h := range hits {
		switch h.kind {
		case "marker":
			locs := make([]string, len(h.markers))
			for i, m := range h.markers {
				locs[i] = fmt.Sprintf("%s:%d", m.file, m.line)
			}
			fmt.Fprintf(os.Stderr, "  %s — marker at %s\n", h.ruleID, strings.Join(locs, ", "))
		case "owns":
			fmt.Fprintf(os.Stderr, "  %s — owns %s — rules %s\n", h.doc, capList(h.files), capList(h.ruleIDs))
		}
	}
	fmt.Fprintln(os.Stderr, "confirm the docs still match")
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
	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("impact", err)
	}
	if len(files) == 0 {
		files, err = stagedPaths(root)
		if err != nil {
			return output.Fail("impact", err)
		}
	}
	hits, err := buildImpact(workingTreeRoots(root), docsRoot, files, m.list(pathsExcludeKey))
	if err != nil {
		return output.Fail("impact", err)
	}
	printImpact(hits)
	return 0
}
