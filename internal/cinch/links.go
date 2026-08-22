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
	var rep linksReport
	err := walkMarkdownFiles(docsRoot, func(path string) error {
		rep.docs++
		findings, links, ferr := checkLinksInFile(path)
		rep.links += links
		rep.findings = append(rep.findings, findings...)
		return ferr
	})
	if err != nil {
		rep.findings = append(rep.findings, scanErrorFinding("links", docsRoot, err)...)
	}
	return rep
}

func linksCheckResult(rep linksReport) checkResult {
	if rep.docs == 0 && len(rep.findings) == 0 {
		return checkResult{noOp: "no markdown files found — links check enforces nothing"}
	}
	return checkResult{
		findings: rep.findings,
		detail: fmt.Sprintf("(%d %s, %d %s checked)",
			rep.docs, output.Plural(rep.docs, "doc"),
			rep.links, output.Plural(rep.links, "link")),
	}
}

func checkLinksInFile(path string) ([]finding, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	var findings []finding
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
				findings = append(findings, finding{
					check:   "links",
					level:   "error",
					file:    path,
					line:    lineNo,
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
			line:    lineNo,
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
