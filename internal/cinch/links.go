package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cinch/internal/output"
)

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

type linksReport struct {
	Findings []Finding
	Docs     int
	Links    int
}

func checkLinks(root string) linksReport {
	var rep linksReport
	err := walkMarkdownFiles(root, func(path string) error {
		rep.Docs++
		findings, links, ferr := checkLinksInFile(path)
		rep.Links += links
		rep.Findings = append(rep.Findings, findings...)
		return ferr
	})
	if err != nil {
		rep.Findings = append(rep.Findings, scanErrorFinding("links", root, err)...)
	}
	return rep
}

func linksCheckResult(rep linksReport) checkResult {
	if rep.Docs == 0 && len(rep.Findings) == 0 {
		return checkResult{NoOp: "no markdown files found — links check enforces nothing"}
	}
	return checkResult{
		Findings: rep.Findings,
		Detail: fmt.Sprintf("(%d %s, %d %s checked)",
			rep.Docs, output.Plural(rep.Docs, "doc"),
			rep.Links, output.Plural(rep.Links, "link")),
	}
}

func checkLinksInFile(path string) ([]Finding, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	var findings []Finding
	links := 0
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
			links++
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

	return findings, links, nil
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
