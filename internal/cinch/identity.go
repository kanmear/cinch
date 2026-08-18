package cinch

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type identityResult struct {
	Findings []Finding
	NoOp     string
}

func checkIdentity(docsDir, repoRoot string) identityResult {
	if !isGitRepo(repoRoot) {
		return identityResult{NoOp: "not a git repository"}
	}
	if !hasHead(repoRoot) {
		return identityResult{NoOp: "no commits yet"}
	}
	if !hasParent(repoRoot) {
		return identityResult{NoOp: "only one commit; no HEAD^ to compare"}
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return identityResult{NoOp: "could not resolve repo root"}
	}
	absDocs, err := filepath.Abs(docsDir)
	if err != nil {
		return identityResult{NoOp: "could not resolve paths.docs"}
	}
	docsDir, repoRoot = absDocs, absRoot

	tombstone, err := tombstonePattern(repoRoot)
	if err != nil {
		return identityResult{Findings: []Finding{{
			Check: "identity", Level: "error", File: manifestPath, Line: 1,
			Message: "rules.tombstone is not a valid regexp: " + err.Error(),
		}}}
	}
	if tombstone == nil {
		return identityResult{NoOp: "rules.tombstone is not set in cinch.yml — opt-in, not configured"}
	}

	paths, err := gitOutputLines(repoRoot, "ls-tree", "-r", "--name-only", "HEAD^", "--", relTo(repoRoot, docsDir))
	if err != nil {
		return identityResult{NoOp: "git ls-tree failed"}
	}

	var result identityResult

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
		headIDs := map[string]bool{}
		if headOK {
			for _, item := range parseRuleItems(path, headContent) {
				headIDs[item.ID] = true
			}
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
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", "HEAD^")
	cmd.Dir = root
	return cmd.Run() == nil
}

func tombstonePattern(root string) (*regexp.Regexp, error) {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return nil, nil
	}
	pattern, ok := m.Vars["rules.tombstone"]
	if !ok || pattern == "" {
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
