package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cinch/internal/output"
)

func indexList(docsRoot string) (string, error) {
	type entry struct {
		rel   string
		title string
	}
	var entries []entry

	collected, err := collectMarkdown(docsRoot, func(path string) ([]entry, error) {
		rel := filepath.ToSlash(relTo(docsRoot, path))
		if strings.HasPrefix(rel, "plans/") {
			return nil, nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		title, _ := titleAndTrigger(string(data))
		if title == "" {
			return nil, nil
		}
		return []entry{{rel: rel, title: title}}, nil
	})
	if err != nil {
		return "", err
	}
	entries = collected

	if len(entries) == 0 {
		return "", fmt.Errorf("cinch render has not run — nothing to show")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s under %s:\n\n", len(entries), output.Plural(len(entries), "doc"), docsRoot)
	for _, e := range entries {
		fmt.Fprintf(&b, "%s — %s\n", e.rel, e.title)
	}
	return b.String(), nil
}

type indexReport struct {
	findings []finding
	docs     int
}

func checkIndex(docsRoot string) indexReport {
	var report indexReport
	err := walkMarkdownFiles(docsRoot, func(path string) error {
		rel := filepath.ToSlash(relTo(docsRoot, path))
		if strings.HasPrefix(rel, "plans/") {
			return nil
		}
		report.docs++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		title, _ := titleAndTrigger(string(data))
		if title == "" {
			report.findings = append(report.findings, finding{
				check: "index", level: "error", file: path, line: 1,
				message: "doc has no title (no '# ' heading) — silently dropped from cinch index",
			})
		}
		return nil
	})
	if err != nil {
		report.findings = append(report.findings, scanErrorFinding("index", docsRoot, err)...)
	}
	return report
}

func indexCheckResult(report indexReport) checkResult {
	if report.docs == 0 && len(report.findings) == 0 {
		return checkResult{noOp: "no markdown files found — index check enforces nothing"}
	}
	return checkResult{
		findings: report.findings,
		detail:   fmt.Sprintf("(%d %s checked)", report.docs, output.Plural(report.docs, "doc")),
	}
}

func CmdIndex(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("index", err)
	}
	list, err := indexList(docsRoot)
	if err != nil {
		return output.Fail("index", err)
	}
	fmt.Print(list)
	return 0
}
