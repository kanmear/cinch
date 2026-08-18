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

type couplingResult struct {
	Findings   []Finding
	NoOp       string
	Suppressed []string
}

func checkCoupling(docsDir, repoRoot, msgFile string) couplingResult {
	if !isGitRepo(repoRoot) {
		return couplingResult{NoOp: "not a git repository"}
	}
	if !hasHead(repoRoot) {
		return couplingResult{NoOp: "no commits yet"}
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return couplingResult{NoOp: "could not resolve repo root"}
	}
	absDocs, err := filepath.Abs(docsDir)
	if err != nil {
		return couplingResult{NoOp: "could not resolve paths.docs"}
	}
	docsDir, repoRoot = absDocs, absRoot

	changed, err := gitChangedFiles(repoRoot)
	if err != nil {
		return couplingResult{NoOp: "git diff failed"}
	}
	if len(changed) == 0 {
		return couplingResult{NoOp: "working tree matches HEAD, nothing to compare"}
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
			return nil
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
				continue
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

func IsGitRepo(root string) bool {
	return isGitRepo(root)
}

func hasHead(root string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", "HEAD")
	cmd.Dir = root
	return cmd.Run() == nil
}

func gitChangedFiles(root string) ([]string, error) {
	tracked, err := gitOutputLines(root, "diff", "HEAD", "--name-only")
	if err != nil {
		return nil, err
	}
	untracked, err := gitOutputLines(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var files []string
	for _, f := range append(tracked, untracked...) {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	return files, nil
}

func gitOutputLines(root string, args ...string) ([]string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func gitShow(root, spec string) (content []byte, ok bool) {
	cmd := exec.Command("git", "show", spec)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	return out, true
}
