package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

// indexList pads the path column when aligned is set, so the titles line up;
// piped output keeps the one-line "path — title" shape.
func indexList(docsRoot string, aligned bool) (string, error) {
	type entry struct {
		rel   string
		title string
	}
	var entries []entry

	collected, err := mdscan.Collect(docsRoot, func(path string) ([]entry, error) {
		rel := filepath.ToSlash(mdscan.RelTo(docsRoot, path))
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
	pathWidth := 0
	if aligned {
		for _, e := range entries {
			pathWidth = max(pathWidth, utf8.RuneCountInString(e.rel))
		}
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "%s%s — %s\n", e.rel, strings.Repeat(" ", max(pathWidth-utf8.RuneCountInString(e.rel), 0)), e.title)
	}
	return b.String(), nil
}

type indexReport struct {
	findings []finding
	docs     int
}

func checkIndex(roots checkRoots, docsRoot string) indexReport {
	var report indexReport
	err := roots.walkDocs(docsRoot, func(path string) error {
		rel := filepath.ToSlash(mdscan.RelTo(docsRoot, path))
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

func CmdIndex(repoRoot string, aligned bool) int {
	docsRoot, err := ResolveDocsRoot(repoRoot)
	if err != nil {
		return output.Fail("index", err)
	}
	list, err := indexList(docsRoot, aligned)
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
	err := mdscan.WalkMarkdownFiles(docsRoot, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		title, _ := titleAndTrigger(string(data))
		titles[filepath.ToSlash(mdscan.RelTo(docsRoot, path))] = title
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
		verb := "link"
		if len(linked) == 1 {
			verb = "links"
		}
		fmt.Fprintf(&b, "%d %s %s to %s:\n\n", len(linked), output.Plural(len(linked), "doc"), verb, rel)
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

const workflowsSubdir = "workflows"

func listWorkflowNames(docsRoot string) ([]string, error) {
	directory := filepath.Join(docsRoot, workflowsSubdir)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

// workflowsTable is the markdown table agents read when output is piped. On a
// terminal (aligned) it is a name column and a trigger wrapped to width with a
// hanging indent. The caller decides which, so tests can cover both without a
// pty.
func workflowsTable(docsRoot string, aligned bool, width int) (string, error) {
	names, err := listWorkflowNames(docsRoot)
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("cinch render has not run — nothing to show")
	}

	triggers := make([]string, len(names))
	nameWidth := 0
	for i, name := range names {
		data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, name+".md"))
		if err != nil {
			return "", err
		}
		_, triggers[i] = titleAndTrigger(string(data))
		nameWidth = max(nameWidth, utf8.RuneCountInString(name))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d %s:\n\n", len(names), output.Plural(len(names), "workflow"))
	if !aligned {
		b.WriteString("| Workflow | Trigger |\n")
		b.WriteString("|---|---|\n")
		for i, name := range names {
			file := name + ".md"
			fmt.Fprintf(&b, "| [%s](%s) | %s |\n", file, file, triggers[i])
		}
		return b.String(), nil
	}

	const gap = 2
	indent := strings.Repeat(" ", nameWidth+gap)
	for i, name := range names {
		lines := wrapWords(triggers[i], max(width-nameWidth-gap, minTriggerWidth))
		fmt.Fprintf(&b, "%-*s%s\n", nameWidth+gap, name, lines[0])
		for _, line := range lines[1:] {
			fmt.Fprintf(&b, "%s%s\n", indent, line)
		}
	}
	return b.String(), nil
}

// minTriggerWidth keeps a very narrow terminal from wrapping a trigger one
// word per line.
const minTriggerWidth = 20

// wrapWords breaks text at spaces into lines of at most width runes; a single
// word longer than width gets a line to itself. It always returns one line.
func wrapWords(text string, width int) []string {
	var lines []string
	current := ""
	for _, word := range strings.Fields(text) {
		switch {
		case current == "":
			current = word
		case utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}

func CmdWorkflows(repoRoot string, aligned bool, width int) int {
	docsRoot, err := ResolveDocsRoot(repoRoot)
	if err != nil {
		return output.Fail("workflows", err)
	}
	table, err := workflowsTable(docsRoot, aligned, width)
	if err != nil {
		return output.Fail("workflows", err)
	}
	fmt.Print(table)
	return 0
}

func CmdWorkflow(root, name string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("workflow", err)
	}
	names, err := listWorkflowNames(docsRoot)
	if err != nil {
		return output.Failf("workflow", "cinch render has not run — nothing to show")
	}
	if !slices.Contains(names, name) {
		return output.Failf("workflow", "%q is not a known workflow — run 'cinch workflows' to see what's available", name)
	}
	data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, name+".md"))
	if err != nil {
		return output.Fail("workflow", err)
	}
	fmt.Print(string(data))
	return 0
}
