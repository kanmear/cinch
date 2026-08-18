package cinch

import (
	"path/filepath"
	"regexp"
	"strings"
)

func checkIdentity(docsDir, repoRoot string) checkResult {
	if !isGitRepo(repoRoot) {
		return checkResult{NoOp: "not a git repository"}
	}
	if !hasHead(repoRoot) {
		return checkResult{NoOp: "no commits yet"}
	}
	if !hasParent(repoRoot) {
		return checkResult{NoOp: "only one commit; no HEAD^ to compare"}
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return checkResult{NoOp: "could not resolve repo root"}
	}
	absDocs, err := filepath.Abs(docsDir)
	if err != nil {
		return checkResult{NoOp: "could not resolve paths.docs"}
	}
	docsDir, repoRoot = absDocs, absRoot

	tombstone, err := tombstonePattern(repoRoot)
	if err != nil {
		return checkResult{Findings: []Finding{{
			Check: "identity", Level: "error", File: manifestPath, Line: 1,
			Message: "rules.tombstone is not a valid regexp: " + err.Error(),
		}}}
	}
	if tombstone == nil {
		return checkResult{NoOp: "rules.tombstone is not set in cinch.yml — opt-in, not configured"}
	}

	paths, err := gitOutputLines(repoRoot, "ls-tree", "-r", "--name-only", "HEAD^", "--", relTo(repoRoot, docsDir))
	if err != nil {
		return checkResult{NoOp: "git ls-tree failed"}
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
			if headIDs[item.ID] {
				continue
			}
			if headOK && idIsTombstoned(tombstone, headContent, item.ID) {
				continue
			}
			result.Findings = append(result.Findings, Finding{
				Check: "identity", Level: "error", File: item.File, Line: item.Line,
				Message: item.ID + ": rule ID present at HEAD^ but absent at HEAD with no recognized tombstone (set rules.tombstone in cinch.yml, or restore the ID)",
			})
		}
	}

	return result
}

func hasParent(root string) bool {
	return gitRun(root, "rev-parse", "--verify", "-q", "HEAD^") == nil
}

func tombstonePattern(root string) (*regexp.Regexp, error) {
	pattern, ok, _ := manifestSetting(root, "rules.tombstone")
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
