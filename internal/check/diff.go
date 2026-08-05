package check

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cinch/internal/manifest"
)

// C10 — diff-coupling (D012, D065). A change that touches paths under a
// domain's owns: front-matter list but not that domain's domain doc is a
// potential rule falsification: behaviour changed, the rule text may now be
// false, and the semantic audit's question is cheapest right now. Warn-level:
// most commits legitimately don't change rules. The boundary is the working
// tree vs HEAD — index and worktree both, so the question fires before the
// commit is made and self-corrects when code and doc land in the same change;
// on a clean tree the diff is empty and C10 is silent. Not a git repo, no
// HEAD yet, or no git binary → skip: absence is a declaration, same as
// C5/C6/C7/C12. Lateral under D009: it binds a change set to the domain doc
// and validates no doc's copy of a machine-readable fact.
//
// The working-tree window is deliberate — C10 is a hook-only check: the
// question is asked before the commit at the moment the answer is cheapest,
// and committed drift at the domain level is not this check's class. C14
// covers the rule-level committed case (D082).
func checkDiffCoupling(root string, m *manifest.Manifest, r *report) {
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		return
	}
	dir := filepath.Join(root, bdir)

	domains := map[string][]string{} // domain name -> owns entries
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "overview.md" {
			return nil
		}
		if owns := ownsList(readFile(p)); len(owns) > 0 {
			domains[strings.TrimSuffix(d.Name(), ".md")] = owns
		}
		return nil
	})
	if len(domains) == 0 {
		return
	}

	changed, err := gitChangedPaths(root)
	if err != nil || len(changed) == 0 {
		return
	}
	changedSet := map[string]bool{}
	for _, p := range changed {
		changedSet[p] = true
	}

	for _, name := range manifest.SortedKeys(domains) {
		var touched []string
		for _, p := range changed {
			for _, own := range domains[name] {
				own = strings.TrimRight(own, "/")
				if p == own || strings.HasPrefix(p, own+"/") {
					touched = append(touched, p)
					break
				}
			}
		}
		if len(touched) == 0 {
			continue
		}
		doc := filepath.Join(bdir, name+".md")
		if changedSet[doc] {
			continue
		}
		sample := touched
		if len(sample) > 3 {
			sample = append(sample[:3], fmt.Sprintf("%d more", len(touched)-3))
		}
		r.warnf("C10", "change touches %s (under %s owns:) but %s is unchanged — if behaviour changed, the rules need updating",
			strings.Join(sample, ", "), name, doc)
	}
}

// gitChangedPaths returns the repo-relative paths changed in the working tree
// (index + worktree) vs HEAD: `git diff HEAD --name-only`. A rename surfaces
// by its new path. Non-git roots and unborn HEADs fail and the check skips —
// absence is a declaration.
func gitChangedPaths(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "diff", "HEAD", "--name-only")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// C14 — rule-level diff-coupling (D082). Warn when a rule's text changes and
// the file carrying that rule's // cinch:rule marker does not — C10's shape
// one level down: C10 binds a change set to the domain doc, C14 binds a rule
// text to its marked test. Two ref-free windows:
//   - working tree vs HEAD (C10's window): rule text differs between HEAD's
//     doc and the current doc, marker file not among the working-tree change
//     set — asked before the commit, at the moment the answer is cheapest;
//   - the last commit that touched the rule's doc (`git log -1 -- <doc>`):
//     rule text differs between that commit and its parent, marker file not
//     in that commit's change set — the committed-drift class the P5.6 audit
//     missed (rule text changed, test unchanged).
//
// Persistent warn: a committed rule-text change without its marker stays
// flagged until the rule's doc is touched again — the falsifying commit is
// the fact. Warn-level, same philosophy as C10: a rule-text clarification
// with no test change is legitimate; the value is asking the question. Rules
// with no marker are C8's domain (marker-less and cinch:ignore rules drop out
// here); a rule newly given an ID is not a text change (C8 owns the closure).
// Lateral under D009: binds two authored artifacts.
func checkRuleDiffCoupling(root string, m *manifest.Manifest, r *report) {
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		return
	}
	dir := filepath.Join(root, bdir)

	markerFiles := scanRuleMarkers(root)

	changed, _ := gitChangedPaths(root)
	changedSet := map[string]bool{}
	for _, p := range changed {
		changedSet[p] = true
	}

	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "overview.md" {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		cur := parseDomainDoc(readFile(p)).rules
		if len(cur) == 0 {
			return nil
		}

		// Working-tree window: HEAD's doc vs the current doc.
		headText, err := gitShow(root, "HEAD", rel)
		if err == nil {
			for _, id := range changedRules(parseDomainDoc(headText).rules, cur) {
				marker := markerFiles[id]
				if marker != "" && !changedSet[marker] {
					r.warnf("C14", "rule %s in %s changed in the working tree but its marker file %s did not — does the test still enforce it?", id, rel, marker)
				}
			}
		}

		// Committed window: the last commit that touched the doc vs its parent.
		last, err := gitLogLast(root, rel)
		if err != nil || last == "" {
			return nil
		}
		oldText, err := gitShow(root, last+"^", rel)
		if err != nil {
			return nil // first commit, or the doc was renamed into this path
		}
		newText, err := gitShow(root, last, rel)
		if err != nil {
			return nil
		}
		paths, err := gitDiffPaths(root, last+"^", last)
		if err != nil {
			return nil
		}
		pathSet := map[string]bool{}
		for _, pth := range paths {
			pathSet[pth] = true
		}
		short := last
		if len(short) > 7 {
			short = short[:7]
		}
		for _, id := range changedRules(parseDomainDoc(oldText).rules, parseDomainDoc(newText).rules) {
			marker := markerFiles[id]
			if marker != "" && !pathSet[marker] {
				r.warnf("C14", "rule %s in %s changed in commit %s but its marker file %s did not — does the test still enforce it?", id, rel, short, marker)
			}
		}
		return nil
	})
}

// changedRules returns the rule IDs whose text differs between two parsed
// docs. IDs present on only one side are not text changes: a new ID is C8's
// closure question, a removed ID means the rule is gone.
func changedRules(a, b map[string]string) []string {
	var ids []string
	for id, text := range b {
		if old, ok := a[id]; ok && old != text {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// gitShow returns the content of rel at ref: `git show <ref>:<rel>`.
func gitShow(root, ref, rel string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "show", ref+":"+rel)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// gitLogLast returns the hash of the last commit on HEAD's history touching
// rel, or "" when none exists.
func gitLogLast(root, rel string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "log", "-1", "--format=%H", "--", rel)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// gitDiffPaths returns the paths changed between two refs.
func gitDiffPaths(root, a, b string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "diff", a, b, "--name-only")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}
