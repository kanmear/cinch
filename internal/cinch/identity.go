package cinch

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// identityResult separates real findings from the no-op cases: not a git
// repo, or only one commit (no HEAD^ to compare against) — silence must
// never read as a pass.
type identityResult struct {
	Findings []Finding
	NoOp     string
}

// checkIdentity fires when a rule ID present at HEAD^ is absent at HEAD with
// no recognized tombstone in its place. Direction: doc-upstream — a rule
// doc's own convention (append-only, tombstoned in place when retired) is
// the spec; this checks the doc still conforms to it across the most recent
// commit. Opt-in: absent rules.tombstone, the check no-ops rather than
// assuming a convention it was never told — a project that hasn't declared
// how it tombstones can't have that declaration checked.
//
// This is a transition check like coupling, but one step further back: HEAD^
// vs HEAD rather than the working tree vs HEAD. coupling's window goes dark
// the moment a change is committed; identity picks up exactly there, at the
// cost of only ever seeing the single most recent commit — see README §
// Known limitations.
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
			continue // shouldn't happen: ls-tree just listed it at HEAD^
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

// tombstonePattern reads the optional rules.tombstone manifest key. A nil,
// nil-error return means the key is absent — checkIdentity no-ops in that
// case, the same opt-in-only contract absent commit.pattern and
// require.cinch already have.
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

// idIsTombstoned reports whether any of tombstone's matches against content
// contain id's literal text — cinch supplies the containment check, the
// project supplies the regexp marking its own tombstone convention in
// place (bind a value, not a shape).
func idIsTombstoned(tombstone *regexp.Regexp, content []byte, id string) bool {
	for _, m := range tombstone.FindAllString(string(content), -1) {
		if strings.Contains(m, id) {
			return true
		}
	}
	return false
}
