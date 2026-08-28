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

// docTitleByPath leaves the title empty for an untitled doc rather than
// skipping it the way indexList does — the index check already reports those,
// and a link query still has to be able to name them. Includes plans/, per
// docLinks.
func docTitleByPath(docsRoot string) (map[string]string, error) {
	titles := make(map[string]string)
	err := walkMarkdownFiles(docsRoot, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		title, _ := titleAndTrigger(string(data))
		titles[filepath.ToSlash(relTo(docsRoot, path))] = title
		return nil
	})
	if err != nil {
		return nil, err
	}
	return titles, nil
}

// matchDocPaths treats a path with a directory in it as an address, and a bare
// basename as a question — so every doc answering to it comes back, including
// when one sits at the docs root and matches exactly. That last case is the
// point: preferring the top-level architecture.md would hide the ambiguity the
// caller is asking about.
func matchDocPaths(titles map[string]string, target string) []string {
	target = strings.TrimPrefix(filepath.ToSlash(target), "./")
	if _, ok := titles[target]; ok && strings.Contains(target, "/") {
		return []string{target}
	}
	var matches []string
	for rel := range titles {
		if filepath.Base(rel) == filepath.Base(target) {
			matches = append(matches, rel)
		}
	}
	sort.Strings(matches)
	return matches
}

func describeDoc(rel, title string) string {
	if title == "" {
		return rel
	}
	return rel + " — " + title
}

// ambiguousMatchList carries each candidate's counts in both directions,
// since those are what tell three same-named docs apart — but it closes on the
// direction actually asked for, so the same listing doesn't come back
// identical whichever flag was typed.
func ambiguousMatchList(docsRoot string, titles map[string]string, graph docLinks, matches []string, inbound bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d docs match %q:\n\n", len(matches), filepath.Base(matches[0]))
	for _, rel := range matches {
		fmt.Fprintf(&b, "%s (%d in, %d out)\n",
			describeDoc(rel, titles[rel]), len(graph.inbound[rel]), len(graph.outbound[rel]))
	}
	question := "what it links to"
	if inbound {
		question = "what links to it"
	}
	fmt.Fprintf(&b, "\ngive a path under %s to see %s\n", docsRoot, question)
	return b.String()
}

func linksList(docsRoot, target string, inbound bool) (string, error) {
	titles, err := docTitleByPath(docsRoot)
	if err != nil {
		return "", err
	}
	if len(titles) == 0 {
		return "", fmt.Errorf("no markdown files under %s — run 'cinch render' first", docsRoot)
	}

	// A pasted path often carries the docs root already; accept it either way.
	target = strings.TrimPrefix(filepath.ToSlash(target), filepath.ToSlash(docsRoot)+"/")
	matches := matchDocPaths(titles, target)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%q matches no doc under %s — run 'cinch index' to see what exists", target, docsRoot)
	case 1:
	default:
		return ambiguousMatchList(docsRoot, titles, buildDocLinks(docsRoot), matches, inbound), nil
	}

	rel := matches[0]
	graph := buildDocLinks(docsRoot)
	linked := graph.outbound[rel]
	if inbound {
		linked = graph.inbound[rel]
	}

	var b strings.Builder
	switch {
	case len(linked) == 0 && inbound:
		fmt.Fprintf(&b, "nothing links to %s\n", rel)
		return b.String(), nil
	case len(linked) == 0:
		fmt.Fprintf(&b, "%s links to no docs\n", rel)
		return b.String(), nil
	case inbound:
		fmt.Fprintf(&b, "%d %s link to %s:\n\n", len(linked), output.Plural(len(linked), "doc"), rel)
	default:
		fmt.Fprintf(&b, "%s links to %d %s:\n\n", rel, len(linked), output.Plural(len(linked), "doc"))
	}
	for _, l := range linked {
		fmt.Fprintf(&b, "%s\n", describeDoc(l, titles[l]))
	}
	return b.String(), nil
}

func cmdIndexLinks(repoRoot, target string, inbound bool) int {
	docsRoot, err := ResolveDocsRoot(repoRoot)
	if err != nil {
		return output.Fail("index", err)
	}
	list, err := linksList(docsRoot, target, inbound)
	if err != nil {
		return output.Fail("index", err)
	}
	fmt.Print(list)
	return 0
}

func CmdIndexLinksTo(repoRoot, target string) int {
	return cmdIndexLinks(repoRoot, target, true)
}

func CmdIndexLinksFrom(repoRoot, target string) int {
	return cmdIndexLinks(repoRoot, target, false)
}
