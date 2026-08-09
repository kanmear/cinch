package cinch

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	fenceRe  = regexp.MustCompile("^\\s*```")
)

// checkLinks fires on a relative markdown link under root that doesn't
// resolve to a file on disk. Direction: structural — a doc's own asserted
// cross-reference must resolve; not a copy of a machine-readable fact.
func checkLinks(root string) []Finding {
	var findings []Finding

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // missing docs dir is a quiet pass, not a check error
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		findings = append(findings, checkLinksInFile(path)...)
		return nil
	})

	return findings
}

func checkLinksInFile(path string) []Finding {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var findings []Finding
	inFence := false
	lineNo := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
			target := strings.TrimSpace(m[1])
			if linkTargetIsExempt(target) {
				continue
			}
			if idx := strings.Index(target, "#"); idx >= 0 {
				target = target[:idx]
			}
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(path), target)
			if _, err := os.Stat(resolved); err != nil {
				findings = append(findings, Finding{
					Check:   "links",
					Level:   "error",
					File:    path,
					Line:    lineNo,
					Message: "link target does not resolve: " + target,
				})
			}
		}
	}

	return findings
}

func linkTargetIsExempt(target string) bool {
	if target == "" {
		return true
	}
	if strings.Contains(target, "://") {
		return true
	}
	if strings.HasPrefix(target, "#") {
		return true
	}
	if strings.HasPrefix(target, "mailto:") {
		return true
	}
	return false
}
