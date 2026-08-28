package cinch

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"cinch/internal/concurrency"
	"cinch/internal/gitutil"
	"cinch/internal/mdscan"
)

func checkRetirement(repoRoot, docsRoot string, m *manifest) checkResult {
	if !gitutil.IsRepo(repoRoot) {
		return checkResult{noOp: "not a git repository"}
	}
	if !gitutil.HasHead(repoRoot) {
		return checkResult{noOp: "no commits yet"}
	}
	if !hasParent(repoRoot) {
		return checkResult{noOp: "only one commit; no HEAD^ to compare"}
	}

	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return checkResult{noOp: "could not resolve repo root"}
	}
	absDocsRoot, err := filepath.Abs(docsRoot)
	if err != nil {
		return checkResult{noOp: "could not resolve paths.docs"}
	}
	docsRoot, repoRoot = absDocsRoot, absRepoRoot

	tombstone, err := tombstonePattern(m)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "retirement", level: "error", file: manifestPath, line: 1,
			message: "retirement.pattern is not a valid regexp: " + err.Error(),
		}}}
	}
	if tombstone == nil {
		return checkResult{noOp: "retirement.pattern is not set in cinch.yml — opt-in, not configured"}
	}

	paths, err := gitutil.OutputLines(repoRoot, "ls-tree", "-r", "--name-only", "HEAD^", "--", mdscan.RelTo(repoRoot, docsRoot))
	if err != nil {
		return checkResult{noOp: "git ls-tree failed"}
	}

	var mdPaths []string
	for _, path := range paths {
		if strings.HasSuffix(path, ".md") {
			mdPaths = append(mdPaths, path)
		}
	}

	return checkResult{findings: retirementFindings(repoRoot, mdPaths, tombstone)}
}

// retirementFindingsForFile compares one markdown file's rule items between
// HEAD^ and HEAD, reporting IDs that disappeared without a recognized
// tombstone.
func retirementFindingsForFile(repoRoot, path string, tombstone *regexp.Regexp) []finding {
	parentContent, ok := gitutil.Show(repoRoot, "HEAD^:"+path)
	if !ok {
		return nil
	}
	parentItems := parseRuleItems(path, parentContent)

	headContent, headOK := gitutil.Show(repoRoot, "HEAD:"+path)
	var headIDs map[string]bool
	if headOK {
		headIDs = ruleItemIDs(parseRuleItems(path, headContent))
	}

	var findings []finding
	for _, item := range parentItems {
		if headIDs[item.id] {
			continue
		}
		if headOK && idIsTombstoned(tombstone, headContent, item.id) {
			continue
		}
		findings = append(findings, finding{
			check: "retirement", level: "error", file: item.file, line: item.line,
			message: item.id + ": rule ID present at HEAD^ but absent at HEAD with no recognized tombstone (set retirement.pattern in cinch.yml, or restore the ID)",
		})
	}
	return findings
}

// retirementFindings fans checkRetirement's per-file git-show work across a
// bounded worker pool — each file spawns up to two `git show` subprocesses,
// so a large doc set is fork/exec-bound rather than CPU-bound. Result order
// is irrelevant: runChecks sorts all findings by (check, file, line) before
// printing.
func retirementFindings(repoRoot string, paths []string, tombstone *regexp.Regexp) []finding {
	if len(paths) == 0 {
		return nil
	}

	jobs := make(chan string)
	results := make(chan []finding)

	var wg sync.WaitGroup
	for i := 0; i < concurrency.BoundedWorkers(len(paths)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				results <- retirementFindingsForFile(repoRoot, path, tombstone)
			}
		}()
	}
	go func() {
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var findings []finding
	for fs := range results {
		findings = append(findings, fs...)
	}
	return findings
}

func hasParent(root string) bool {
	return gitutil.Run(root, "rev-parse", "--verify", "-q", "HEAD^") == nil
}

func tombstonePattern(m *manifest) (*regexp.Regexp, error) {
	pattern, ok := manifestSetting(m, "retirement.pattern")
	if !ok {
		return nil, nil
	}
	return regexp.Compile(pattern)
}

func idIsTombstoned(tombstone *regexp.Regexp, content []byte, id string) bool {
	for _, m := range tombstone.FindAllString(string(content), -1) {
		if strings.Contains(m, id) {
			return true
		}
	}
	return false
}
