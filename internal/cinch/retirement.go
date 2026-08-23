package cinch

import (
	"path/filepath"
	"regexp"
	"strings"
)

func checkRetirement(repoRoot, docsRoot string) checkResult {
	if !isGitRepo(repoRoot) {
		return checkResult{noOp: "not a git repository"}
	}
	if !hasHead(repoRoot) {
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

	tombstone, err := tombstonePattern(repoRoot)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "retirement", level: "error", file: manifestPath, line: 1,
			message: "retirement.pattern is not a valid regexp: " + err.Error(),
		}}}
	}
	if tombstone == nil {
		return checkResult{noOp: "retirement.pattern is not set in cinch.yml — opt-in, not configured"}
	}

	paths, err := gitOutputLines(repoRoot, "ls-tree", "-r", "--name-only", "HEAD^", "--", relTo(repoRoot, docsRoot))
	if err != nil {
		return checkResult{noOp: "git ls-tree failed"}
	}

	var result checkResult

	for _, path := range paths {
		if !strings.HasSuffix(path, ".md") {
			continue
		}

		parentContent, ok := gitShow(repoRoot, "HEAD^:"+path)
		if !ok {
			continue
		}
		parentItems := parseRuleItems(path, parentContent)

		headContent, headOK := gitShow(repoRoot, "HEAD:"+path)
		var headIDs map[string]bool
		if headOK {
			headIDs = ruleItemIDs(parseRuleItems(path, headContent))
		}

		for _, item := range parentItems {
			if headIDs[item.id] {
				continue
			}
			if headOK && idIsTombstoned(tombstone, headContent, item.id) {
				continue
			}
			result.findings = append(result.findings, finding{
				check: "retirement", level: "error", file: item.file, line: item.line,
				message: item.id + ": rule ID present at HEAD^ but absent at HEAD with no recognized tombstone (set retirement.pattern in cinch.yml, or restore the ID)",
			})
		}
	}

	return result
}

func hasParent(root string) bool {
	return gitRun(root, "rev-parse", "--verify", "-q", "HEAD^") == nil
}

func tombstonePattern(root string) (*regexp.Regexp, error) {
	pattern, ok, _ := manifestSetting(root, "retirement.pattern")
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
