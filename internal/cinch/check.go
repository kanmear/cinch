package cinch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"cinch/internal/output"
)

type finding struct {
	check   string
	level   string
	file    string
	line    int
	message string
}

type checkResult struct {
	findings []finding
	noOp     string
	detail   string
}

// checkRoots separates where git runs from where checked content is read.
// git never runs inside an index snapshot; stagedSnapshot says why.
type checkRoots struct {
	repoRoot   string   // real repository: git commands, rules.roots siblings, core.hooksPath
	fsRoot     string   // where file content is read: == repoRoot, or an index snapshot
	indexFiles []string // staged mode only: `git ls-files --cached`, repo-relative
}

func scanErrorFinding(check, scope string, err error) []finding {
	return []finding{{
		check: check, level: "error", file: scope, line: 1,
		message: "failed to scan: " + err.Error(),
	}}
}

func CmdCheck(messageFile string, changed, staged bool, only []string) int {
	if !staged {
		return runChecks(workingTreeRoots("."), messageFile, changed, only...)
	}
	roots, cleanup, err := stagedSnapshot(".")
	defer cleanup()
	if err != nil {
		return output.Fail("check", err)
	}
	return runChecks(roots, messageFile, false, only...)
}

func workingTreeRoots(repoRoot string) checkRoots {
	return checkRoots{repoRoot: repoRoot, fsRoot: repoRoot}
}

// relabel maps a finding read from an index snapshot back to the path it
// has in the repository, so output never names the temporary directory and
// reads exactly as it would for the same content in the working tree.
func (roots checkRoots) relabel(f finding) finding {
	if roots.fsRoot == roots.repoRoot {
		return f
	}
	prefix := roots.fsRoot + string(filepath.Separator)
	replacement := ""
	if roots.repoRoot != "." {
		replacement = roots.repoRoot + string(filepath.Separator)
	}
	switch {
	case f.file == roots.fsRoot:
		f.file = roots.repoRoot
	case strings.HasPrefix(f.file, prefix):
		f.file = replacement + strings.TrimPrefix(f.file, prefix)
	}
	f.message = strings.ReplaceAll(f.message, prefix, replacement)
	return f
}

const (
	hooksPreCommitChecksKey = "hooks.pre-commit-checks"
	hooksCommitMsgChecksKey = "hooks.commit-msg-checks"
)

var (
	defaultPreCommitChecks = []string{"links", "rules", "index", "generated", "core"}
	defaultCommitMsgChecks = []string{"commit"}

	// checkLaunchOrder is both the order checks start in and the order their
	// status lines print in, so the two can't drift apart. Checks finish in
	// whatever order the scheduler picks; printing in this order is what
	// keeps the output stable run to run.
	checkLaunchOrder = []string{"links", "rules", "index", "generated", "commit", "core", "hooks"}
)

// preCommitChecks and commitMsgChecks each load their own manifest copy
// (mirroring runChecks/ResolveDocsRoot's existing style of reloading rather
// than threading a manifest through every call site) purely to look up an
// optional override of their baseline check list — hooks.pre-commit-checks /
// hooks.commit-msg-checks. A malformed cinch.yml is deliberately not an
// error here: it's surfaced properly by runChecks's own load via the
// "generated" check, and this lookup silently falls back to the hardcoded
// default on any error, same as an absent key.
//
// preCommitChecks reads the override from roots.fsRoot, so in staged mode the
// cinch.yml being committed decides which checks gate that commit.
func preCommitChecks(roots checkRoots) int {
	m, _ := loadManifestOptional(roots.fsRoot)
	only := m.list(hooksPreCommitChecksKey)
	if len(only) == 0 {
		only = defaultPreCommitChecks
	}
	return runChecks(roots, "", false, only...)
}

func commitMsgChecks(root, messageFile string) int {
	m, _ := loadManifestOptional(root)
	only := m.list(hooksCommitMsgChecksKey)
	if len(only) == 0 {
		only = defaultCommitMsgChecks
	}
	return runChecks(workingTreeRoots(root), messageFile, false, only...)
}

// changedFileSet resolves the set of unstaged working-tree file paths, keyed
// in the same root-joined coordinate space that every checker's
// finding.file already uses — including when root is an absolute test temp
// dir, since git itself always reports paths relative to the repo root
// regardless of how root was spelled when invoking it.
func changedFileSet(root string) (map[string]bool, error) {
	paths, err := unstagedPaths(root)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[filepath.Join(root, p)] = true
	}
	return set, nil
}

func filterFindingsToChanged(findings []finding, changed map[string]bool) []finding {
	var out []finding
	for _, f := range findings {
		if changed[f.file] {
			out = append(out, f)
		}
	}
	return out
}

// changedUnder reports whether any changed path sits under dir.
func changedUnder(changed map[string]bool, dir string) bool {
	if dir == "" {
		return false
	}
	prefix := dir + "/"
	for f := range changed {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// changedMarkdownUnder reports whether any changed .md file sits under dir,
// outside of the given excludeSubdirs (each relative to dir).
func changedMarkdownUnder(changed map[string]bool, dir string, excludeSubdirs ...string) bool {
	prefix := dir + "/"
	for f := range changed {
		if !strings.HasSuffix(f, ".md") || !strings.HasPrefix(f, prefix) {
			continue
		}
		excluded := false
		for _, sub := range excludeSubdirs {
			if strings.HasPrefix(f, filepath.Join(dir, sub)+"/") {
				excluded = true
				break
			}
		}
		if !excluded {
			return true
		}
	}
	return false
}

// generatedRelevant reports whether anything that can affect checkGenerated's
// verdict changed: the manifest itself (its edits can invalidate every
// destination file at once, e.g. a paths.hooks rename) or a file under one of
// the rendered destination directories.
func generatedRelevant(root, docsRoot string, m *manifest, changed map[string]bool) bool {
	if changed[filepath.Join(root, manifestPath)] {
		return true
	}
	if changedUnder(changed, filepath.Join(docsRoot, workflowsSubdir)) {
		return true
	}
	return changedUnder(changed, filepath.Join(root, hooksPathValue(m)))
}

// runChecks reads every checked file under roots.fsRoot. changed is only
// meaningful for working-tree roots: unstaged paths say nothing about a
// snapshot of the index.
func runChecks(roots checkRoots, messageFile string, changed bool, only ...string) int {
	docsRoot, err := ResolveDocsRoot(roots.fsRoot)
	if err != nil {
		return output.Fail("check", err)
	}

	m, mErr := loadManifestOptional(roots.fsRoot)

	onlySet := make(map[string]bool, len(only))
	for _, n := range only {
		onlySet[n] = true
	}
	run := func(name string) bool { return len(only) == 0 || onlySet[name] }

	var changedSet map[string]bool
	if changed {
		changedSet, err = changedFileSet(roots.repoRoot)
		if err != nil {
			return output.Failf("check", "git diff failed: %s", err.Error())
		}
	}
	// relevant reports whether a check should run at all: always true unless
	// --changed is active, in which case it defers to the check's own
	// changed-scope predicate.
	relevant := func(inScope bool) bool { return !changed || inScope }
	// scoped filters a checker's findings down to changed files — only valid
	// for checks whose findings are always attributable to a single file that
	// itself would need to change to matter (links/index/rules); generated's
	// findings can be caused by an edit to a file other than the one being
	// reported on, so it is gated by generatedRelevant instead and never
	// filtered per-finding.
	scoped := func(fn func() checkResult) func() checkResult {
		return func() checkResult {
			r := fn()
			if changed {
				r.findings = filterFindingsToChanged(r.findings, changedSet)
			}
			return r
		}
	}

	type plannedCheck struct {
		enabled bool
		fn      func() checkResult
	}
	type namedResult struct {
		name   string
		result checkResult
	}

	planned := map[string]plannedCheck{
		"links": {relevant(changedMarkdownUnder(changedSet, docsRoot)),
			scoped(func() checkResult { return linksCheckResult(checkLinks(docsRoot)) })},
		"rules": {relevant(len(changedSet) > 0),
			scoped(func() checkResult { return rulesCheckResult(checkRules(roots, docsRoot, m.list(rulesRootsKey))) })},
		"index": {relevant(changedMarkdownUnder(changedSet, docsRoot, "plans")),
			scoped(func() checkResult { return indexCheckResult(checkIndex(docsRoot)) })},
		"generated": {relevant(generatedRelevant(roots.fsRoot, docsRoot, m, changedSet)),
			func() checkResult { return checkGenerated(roots, m, mErr) }},
		// commit is unconditionally suppressed without a MSGFILE, not just under
		// --changed: nobody hand-invokes `cinch check somefile.txt` in ordinary
		// use (the commit-msg hook goes through commitMsgChecks instead), so
		// "no commit message file given" is noise on every plain `cinch check`,
		// not a diff-scoping question.
		"commit": {messageFile != "", func() checkResult { return checkCommit(messageFile, m) }},
		"core":   {true, func() checkResult { return checkPin(Version, m) }},
		"hooks":  {true, func() checkResult { return checkHooks(roots.repoRoot, m, mErr) }},
	}

	results := make(chan namedResult, len(checkLaunchOrder))
	launched := 0
	for _, name := range checkLaunchOrder {
		check := planned[name]
		if !run(name) || !check.enabled {
			continue
		}
		launched++
		go func() {
			results <- namedResult{name: name, result: check.fn()}
		}()
	}

	finished := make(map[string]checkResult, launched)
	for i := 0; i < launched; i++ {
		r := <-results
		finished[r.name] = r.result
	}

	var findings []finding
	for _, name := range checkLaunchOrder {
		result, ok := finished[name]
		if !ok {
			continue
		}
		for _, f := range result.findings {
			findings = append(findings, roots.relabel(f))
		}
		output.CheckStatus(name, len(result.findings), result.noOp, result.detail)
	}

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.check != b.check {
			return a.check < b.check
		}
		if a.file != b.file {
			return a.file < b.file
		}
		return a.line < b.line
	})

	for _, f := range findings {
		fmt.Printf("%s %s %s:%d: %s\n", f.check, output.Level(f.level), f.file, f.line, f.message)
	}

	if len(findings) > 0 {
		return 1
	}
	return 0
}
