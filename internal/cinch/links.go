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
	findings []finding
	docs     int
	links    int
}

func checkLinks(docsRoot string) linksReport {
	var report linksReport
	err := walkMarkdownFiles(docsRoot, func(path string) error {
		report.docs++
		findings, links, ferr := checkLinksInFile(path)
		report.links += links
		report.findings = append(report.findings, findings...)
		return ferr
	})
	if err != nil {
		report.findings = append(report.findings, scanErrorFinding("links", docsRoot, err)...)
	}
	return report
}

func linksCheckResult(report linksReport) checkResult {
	if report.docs == 0 && len(report.findings) == 0 {
		return checkResult{noOp: "no markdown files found — links check enforces nothing"}
	}
	return checkResult{
		findings: report.findings,
		detail: fmt.Sprintf("(%d %s, %d %s checked)",
			report.docs, output.Plural(report.docs, "doc"),
			report.links, output.Plural(report.links, "link")),
	}
}

func checkLinksInFile(path string) ([]finding, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	var findings []finding
	links := 0
	lineNumber := 0
	scanErr := forEachFencedLine(data, func(n int, line string) {
		lineNumber = n
		for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
			target := strings.TrimSpace(m[1])
			if linkTargetIsExempt(target) {
				continue
			}
			if index := strings.Index(target, "#"); index >= 0 {
				target = target[:index]
			}
			if target == "" {
				continue
			}
			links++
			resolved := filepath.Join(filepath.Dir(path), target)
			if _, err := os.Stat(resolved); err != nil {
				findings = append(findings, finding{
					check:   "links",
					level:   "error",
					file:    path,
					line:    lineNumber,
					message: "link target does not resolve: " + target,
				})
			}
		}
	})
	if scanErr != nil {
		findings = append(findings, finding{
			check:   "links",
			level:   "error",
			file:    path,
			line:    lineNumber,
			message: "failed to scan file: " + scanErr.Error(),
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
