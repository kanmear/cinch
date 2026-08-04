package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
func checkDiffCoupling(root string, m *Manifest, r *report) {
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

	for _, name := range sortedKeys(domains) {
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
