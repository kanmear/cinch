package cinch

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var ruleRewordRe = regexp.MustCompile(`^\s*rule-reword:\s*([A-Z0-9]+-[0-9]+)\s*$`)

// couplingResult separates real findings from the two things that must be
// said explicitly rather than folded into "0 findings": the check didn't run
// at all (no-op), or a finding was suppressed by the escape hatch. Silence
// must never read as a pass.
type couplingResult struct {
	Findings   []Finding
	NoOp       string
	Suppressed []string
}

// checkCoupling fires when a rule's item-scoped text changed in the working
// tree vs HEAD and the file holding that rule's // cinch:rule marker is not
// among the changed files. Direction: lateral — principle 1's own named
// pattern ("this text changed and its enforcing file did not").
//
// This is a transition check: its window is the working tree against HEAD,
// so it is invisible post-commit (nothing changed, nothing to compare).
func checkCoupling(docsDir, repoRoot, msgFile string) couplingResult {
	if !isGitRepo(repoRoot) {
		return couplingResult{NoOp: "coupling: not a git repository — check did not run"}
	}
	if !hasHead(repoRoot) {
		return couplingResult{NoOp: "coupling: no commits yet — check did not run"}
	}

	changed, err := gitChangedFiles(repoRoot)
	if err != nil {
		return couplingResult{NoOp: "coupling: git diff failed — check did not run"}
	}
	if len(changed) == 0 {
		return couplingResult{NoOp: "coupling: working tree matches HEAD — nothing to compare, check did not run"}
	}
	changedSet := map[string]bool{}
	for _, f := range changed {
		changedSet[f] = true
	}

	markers := scanRuleMarkers(repoRoot, docsDir)
	escaped := parseRuleReword(msgFile)

	var result couplingResult

	_ = filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		headContent, ok := gitShow(repoRoot, "HEAD:"+relTo(repoRoot, path))
		if !ok {
			return nil // new file, nothing at HEAD to diff against
		}
		headByID := map[string]string{}
		for _, item := range parseRuleItems(path, headContent) {
			headByID[item.ID] = item.Text
		}

		for _, wi := range parseRuleDoc(path) {
			headText, existed := headByID[wi.ID]
			if !existed || headText == wi.Text {
				continue
			}

			locs, hasMarker := markers[wi.ID]
			if !hasMarker {
				continue // no marker: rules check's job, not coupling's
			}
			markerChanged := false
			for _, loc := range locs {
				if changedSet[relTo(repoRoot, loc.File)] {
					markerChanged = true
					break
				}
			}
			if markerChanged {
				continue
			}

			if escaped[wi.ID] {
				result.Suppressed = append(result.Suppressed,
					wi.ID+" suppressed by rule-reword")
				continue
			}

			result.Findings = append(result.Findings, Finding{
				Check: "coupling", Level: "block", File: wi.File, Line: wi.Line,
				Message: wi.ID + ": rule text changed but marked test file (" + locs[0].File +
					") did not — escape via `rule-reword: " + wi.ID + "` in the commit message",
			})
		}
		return nil
	})

	return result
}

func parseRuleReword(msgFile string) map[string]bool {
	escaped := map[string]bool{}
	if msgFile == "" {
		return escaped
	}
	data, err := os.ReadFile(msgFile)
	if err != nil {
		return escaped
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if m := ruleRewordRe.FindStringSubmatch(scanner.Text()); m != nil {
			escaped[m[1]] = true
		}
	}
	return escaped
}

// relTo returns path relative to base (best-effort: git's own output —
// gitChangedFiles — is always repo-relative, but the filesystem-walk paths
// this check builds take whatever form the caller's repoRoot/docsDir had,
// which is absolute in tests and "." in production; the two must be
// normalized to the same form before comparing).
func relTo(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

func isGitRepo(root string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func hasHead(root string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", "HEAD")
	cmd.Dir = root
	return cmd.Run() == nil
}

func gitChangedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "diff", "HEAD", "--name-only")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// gitShow returns a file's content at the given revision-and-path spec
// (e.g. "HEAD:.docs/rules.md"), or ok=false if it doesn't exist there.
func gitShow(root, spec string) (content []byte, ok bool) {
	cmd := exec.Command("git", "show", spec)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	return out, true
}
