package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cinch/internal/output"
)

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

type linksReport struct {
	findings []finding
	edges    []docEdge
	docs     int
	links    int
}

// docEdge is one resolved markdown link, both ends as slash-separated
// docs-root-relative paths. Storing the resolution rather than the link text
// is what lets a caller tell which of three same-named docs a
// "../architecture.md" points at.
type docEdge struct {
	from string
	to   string
}

// docLinks is the doc-to-doc link graph, indexed both ways. Unlike indexList,
// it covers plans/ as both source and target: that exclusion answers "what is
// the doc corpus", while "what links here" would be lying by omission if it
// dropped a whole directory of referrers.
type docLinks struct {
	inbound  map[string][]string
	outbound map[string][]string
}

func checkLinks(docsRoot string) linksReport {
	var report linksReport
	err := walkMarkdownFiles(docsRoot, func(path string) error {
		report.docs++
		fileReport, ferr := checkLinksInFile(docsRoot, path)
		report.findings = append(report.findings, fileReport.findings...)
		report.edges = append(report.edges, fileReport.edges...)
		report.links += fileReport.links
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

// checkLinksInFile returns a partial linksReport: docs stays zero, since a
// single file is always exactly one doc and only the caller is counting.
func checkLinksInFile(docsRoot, path string) (linksReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return linksReport{}, err
	}

	var report linksReport
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
			report.links++
			resolved := filepath.Join(filepath.Dir(path), target)
			if _, err := os.Stat(resolved); err != nil {
				report.findings = append(report.findings, finding{
					check:   "links",
					level:   "error",
					file:    path,
					line:    lineNumber,
					message: "link target does not resolve: " + target,
				})
				continue
			}
			if edge, ok := docEdgeFor(docsRoot, path, resolved); ok {
				report.edges = append(report.edges, edge)
			}
		}
	})
	if scanErr != nil {
		report.findings = append(report.findings, finding{
			check:   "links",
			level:   "error",
			file:    path,
			line:    lineNumber,
			message: "failed to scan file: " + scanErr.Error(),
		})
	}

	return report, nil
}

// docEdgeFor returns the graph edge for a link that resolved on disk. Links
// to code, images, or anything above docsRoot are valid links but not
// doc-to-doc edges; a self-link is dropped because listing a doc among its
// own referrers reads as a bug, not a fact.
func docEdgeFor(docsRoot, from, resolvedTo string) (docEdge, bool) {
	if !strings.HasSuffix(resolvedTo, ".md") {
		return docEdge{}, false
	}
	to := filepath.ToSlash(relTo(docsRoot, resolvedTo))
	if to == ".." || strings.HasPrefix(to, "../") {
		return docEdge{}, false
	}
	source := filepath.ToSlash(relTo(docsRoot, from))
	if source == to {
		return docEdge{}, false
	}
	return docEdge{from: source, to: to}, true
}

// buildDocLinks goes through checkLinks rather than re-parsing, so the graph
// and the links check can never disagree about what resolves to what. The
// findings that come back with the edges are the check's business, not this
// caller's.
func buildDocLinks(docsRoot string) docLinks {
	graph := docLinks{
		inbound:  make(map[string][]string),
		outbound: make(map[string][]string),
	}
	for _, e := range checkLinks(docsRoot).edges {
		graph.outbound[e.from] = append(graph.outbound[e.from], e.to)
		graph.inbound[e.to] = append(graph.inbound[e.to], e.from)
	}
	for _, index := range []map[string][]string{graph.inbound, graph.outbound} {
		for key, paths := range index {
			index[key] = sortedUnique(paths)
		}
	}
	return graph
}

func sortedUnique(paths []string) []string {
	sort.Strings(paths)
	out := paths[:0]
	for i, p := range paths {
		if i == 0 || p != paths[i-1] {
			out = append(out, p)
		}
	}
	return out
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
