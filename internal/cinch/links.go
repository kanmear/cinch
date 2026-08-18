package cinch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func checkLinks(root string) []Finding {
	findings, err := collectMarkdown(root, checkLinksInFile)
	if err != nil {
		return append(findings, scanErrorFinding("links", root, err)...)
	}
	return findings
}

func checkLinksInFile(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var findings []Finding
	lineNo := 0
	scanErr := forEachFencedLine(data, func(n int, line string) {
		lineNo = n
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
	})
	if scanErr != nil {
		findings = append(findings, Finding{
			Check:   "links",
			Level:   "error",
			File:    path,
			Line:    lineNo,
			Message: "failed to scan file: " + scanErr.Error(),
		})
	}

	return findings, nil
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
