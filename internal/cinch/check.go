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

func scanErrorFinding(check, scope string, err error) []finding {
	return []finding{{
		check: check, level: "error", file: scope, line: 1,
		message: "failed to scan: " + err.Error(),
	}}
}

func CmdCheck(messageFile string, changed bool, only []string) int {
	return runChecks(".", messageFile, changed, only...)
}

const (
	hooksPreCommitChecksKey = "hooks.pre-commit-checks"
	hooksCommitMsgChecksKey = "hooks.commit-msg-checks"
)

var (
	defaultPreCommitChecks = []string{"links", "rules", "index", "generated", "core"}
	defaultCommitMsgChecks = []string{"commit"}
)

// preCommitChecks and commitMsgChecks each load their own manifest copy
// (mirroring runChecks/ResolveDocsRoot's existing style of reloading rather
// than threading a manifest through every call site) purely to look up an
// optional override of their baseline check list — hooks.pre-commit-checks /
// hooks.commit-msg-checks. A malformed cinch.yml is deliberately not an
// error here: it's surfaced properly by runChecks's own load via the
// "generated" check, and this lookup silently falls back to the hardcoded
// default on any error, same as an absent key.
func preCommitChecks(root string) int {
	m, _ := loadManifestOptional(root)
	only := m.list(hooksPreCommitChecksKey)
	if len(only) == 0 {
		only = defaultPreCommitChecks
	}
	return runChecks(root, "", false, only...)
}

func commitMsgChecks(root, messageFile string) int {
	m, _ := loadManifestOptional(root)
	only := m.list(hooksCommitMsgChecksKey)
	if len(only) == 0 {
		only = defaultCommitMsgChecks
	}
	return runChecks(root, messageFile, false, only...)
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

func runChecks(root, messageFile string, changed bool, only ...string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("check", err)
	}

	m, mErr := loadManifestOptional(root)

	onlySet := make(map[string]bool, len(only))
	for _, n := range only {
		onlySet[n] = true
	}
	run := func(name string) bool { return len(only) == 0 || onlySet[name] }

	var changedSet map[string]bool
	if changed {
		changedSet, err = changedFileSet(root)
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

	type namedResult struct {
		name   string
		result checkResult
	}

	results := make(chan namedResult, 8)
	launched := 0
	launch := func(name string, ok bool, fn func() checkResult) {
		if !run(name) || !ok {
			return
		}
		launched++
		go func() {
			results <- namedResult{name: name, result: fn()}
		}()
	}

	launch("links", relevant(changedMarkdownUnder(changedSet, docsRoot)),
		scoped(func() checkResult { return linksCheckResult(checkLinks(docsRoot)) }))
	launch("rules", relevant(len(changedSet) > 0),
		scoped(func() checkResult { return rulesCheckResult(checkRules(root, docsRoot, m.list(rulesRootsKey))) }))
	launch("index", relevant(changedMarkdownUnder(changedSet, docsRoot, "plans")),
		scoped(func() checkResult { return indexCheckResult(checkIndex(docsRoot)) }))
	launch("generated", relevant(generatedRelevant(root, docsRoot, m, changedSet)),
		func() checkResult { return checkGenerated(root, m, mErr) })
	// commit is unconditionally suppressed without a MSGFILE, not just under
	// --changed: nobody hand-invokes `cinch check somefile.txt` in ordinary
	// use (the commit-msg hook goes through commitMsgChecks instead), so
	// "no commit message file given" is noise on every plain `cinch check`,
	// not a diff-scoping question.
	launch("commit", messageFile != "", func() checkResult { return checkCommit(messageFile, m) })
	launch("core", true, func() checkResult { return checkPin(Version, m) })
	launch("hooks", true, func() checkResult { return checkHooks(root, m, mErr) })

	var findings []finding
	for i := 0; i < launched; i++ {
		r := <-results
		findings = append(findings, r.result.findings...)
		output.CheckStatus(r.name, len(r.result.findings), r.result.noOp, r.result.detail)
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
