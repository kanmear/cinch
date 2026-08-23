package cinch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"cinch/internal/output"
)

var (
	ruleItemRe = regexp.MustCompile(`^\s*\d+\.\s+\*\*([A-Z0-9]+-[0-9]+)\*\*`)
	anyItemRe  = regexp.MustCompile(`^\s*\d+\.\s`)
	ignoreRe   = regexp.MustCompile(`<!--\s*cinch:ignore\s*(?::\s*(.*?))?\s*-->`)
	markerRe   = regexp.MustCompile(`(?://|#|--|<!--|/\*|%|;)\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b`)
)

type ruleItem struct {
	id   string
	file string
	line int

	text string

	hasIgnore    bool
	ignoreReason string
	ignoreLine   int
}

type markerLoc struct {
	file string
	line int
}

type rulesReport struct {
	findings []finding
	rules    int
	docs     int
	ignores  int
}

func ruleItemIDs(items []ruleItem) map[string]bool {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.id] = true
	}
	return ids
}

func parseRuleItems(file string, content []byte) []ruleItem {
	var items []ruleItem
	var cur *ruleItem
	var textLines []string

	flush := func() {
		if cur == nil {
			return
		}
		cur.text = strings.Join(strings.Fields(strings.Join(textLines, " ")), " ")
		items = append(items, *cur)
		cur = nil
		textLines = nil
	}

	_ = forEachFencedLine(content, func(lineNumber int, line string) {
		if m := ruleItemRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &ruleItem{id: m[1], file: file, line: lineNumber}
			textLines = []string{line}
			return
		}
		if anyItemRe.MatchString(line) {
			flush()
			return
		}
		if cur == nil {
			return
		}
		textLines = append(textLines, line)
		if !cur.hasIgnore {
			if m := ignoreRe.FindStringSubmatch(line); m != nil {
				cur.hasIgnore = true
				cur.ignoreReason = strings.TrimSpace(m[1])
				cur.ignoreLine = lineNumber
			}
		}
	})
	flush()

	return items
}

func parseRuleDoc(path string) ([]ruleItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRuleItems(path, data), nil
}

func scanRuleDocs(docsRoot string) ([]ruleItem, error) {
	return collectMarkdown(docsRoot, parseRuleDoc)
}

func underPath(path, directory string) bool {
	if directory == "" {
		return false
	}
	absPath, err1 := filepath.Abs(path)
	absDirectory, err2 := filepath.Abs(directory)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absDirectory, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func markerScanFiles(repoRoot, docsRoot string) ([]string, error) {
	if files, err := gitScannableFiles(repoRoot); err == nil {
		var out []string
		for _, f := range files {
			if !underPath(f, docsRoot) {
				out = append(out, f)
			}
		}
		return out, nil
	}

	var out []string
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || underPath(path, docsRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if underPath(path, docsRoot) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			rel = path
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func scanMarkers(r io.Reader, path string, fenced bool) (map[string][]markerLoc, error) {
	markers := map[string][]markerLoc{}
	fn := func(lineNumber int, line string) {
		if m := markerRe.FindStringSubmatch(line); m != nil {
			markers[m[1]] = append(markers[m[1]], markerLoc{file: path, line: lineNumber})
		}
	}
	var err error
	if fenced {
		err = forEachFencedLineReader(r, fn)
	} else {
		err = forEachLineReader(r, fn)
	}
	return markers, err
}

func scanFileMarkers(path string) (map[string][]markerLoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return scanMarkers(f, path, strings.HasSuffix(path, ".md"))
}

// scanRuleMarkers reads every in-scope file looking for // cinch:rule
// markers, using a bounded worker pool since each file is an independent
// os.Open + scan. The merge order across files is not guaranteed, so the
// location order within markers[id] is not stable run-to-run when the same
// ID appears in more than one file.
func scanRuleMarkers(repoRoot, docsRoot string) (map[string][]markerLoc, error) {
	files, err := markerScanFiles(repoRoot, docsRoot)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return map[string][]markerLoc{}, nil
	}

	type fileResult struct {
		markers map[string][]markerLoc
		err     error
	}

	jobs := make(chan string)
	results := make(chan fileResult)

	var wg sync.WaitGroup
	for i := 0; i < boundedWorkers(len(files)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				m, err := scanFileMarkers(filepath.Join(repoRoot, file))
				results <- fileResult{markers: m, err: err}
			}
		}()
	}
	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	markers := map[string][]markerLoc{}
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for id, locs := range r.markers {
			markers[id] = append(markers[id], locs...)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return markers, nil
}

func checkRules(repoRoot, docsRoot string) rulesReport {
	items, err := scanRuleDocs(docsRoot)
	if err != nil {
		return rulesReport{findings: scanErrorFinding("rules", docsRoot, err)}
	}
	markers, err := scanRuleMarkers(repoRoot, docsRoot)
	if err != nil {
		return rulesReport{findings: scanErrorFinding("rules", repoRoot, err)}
	}

	return checkRulesFrom(items, markers)
}

func rulesCheckResult(report rulesReport) checkResult {
	if report.rules == 0 && len(report.findings) == 0 {
		return checkResult{noOp: "no rule IDs found — rules check enforces nothing"}
	}
	return checkResult{
		findings: report.findings,
		detail: fmt.Sprintf("(%d %s, %d %s, %d %s)",
			report.rules, output.Plural(report.rules, "rule"),
			report.docs, output.Plural(report.docs, "rule doc"),
			report.ignores, output.Plural(report.ignores, "ignore")),
	}
}

func checkRulesFrom(items []ruleItem, markers map[string][]markerLoc) rulesReport {
	var findings []finding

	for _, item := range items {
		_, marked := markers[item.id]

		if item.hasIgnore && item.ignoreReason == "" {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.ignoreLine,
				message: item.id + ": cinch:ignore has no reason",
			})
		}
		if item.hasIgnore && item.ignoreReason != "" && marked {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.line,
				message: item.id + ": declared cinch:ignore but also has a // cinch:rule marker — pick one",
			})
		}
		if !item.hasIgnore && !marked {
			findings = append(findings, finding{
				check: "rules", level: "error", file: item.file, line: item.line,
				message: item.id + ": no // cinch:rule marker (or cinch:ignore declaration)",
			})
		}
	}

	ruleIDs := ruleItemIDs(items)
	for id, locs := range markers {
		if ruleIDs[id] {
			continue
		}
		for _, loc := range locs {
			findings = append(findings, finding{
				check: "rules", level: "error", file: loc.file, line: loc.line,
				message: "// cinch:rule " + id + " does not resolve to any rule ID",
			})
		}
	}

	docs := make(map[string]bool, len(items))
	ignores := 0
	for _, item := range items {
		docs[item.file] = true
		if item.hasIgnore {
			ignores++
		}
	}

	return rulesReport{
		findings: findings,
		rules:    len(items),
		docs:     len(docs),
		ignores:  ignores,
	}
}

func CmdIgnores(docsRoot string) int {
	items, err := scanRuleDocs(docsRoot)
	if err != nil {
		return output.Fail("ignores", err)
	}
	var ignored []ruleItem
	for _, item := range items {
		if item.hasIgnore {
			ignored = append(ignored, item)
		}
	}
	if len(ignored) == 0 {
		fmt.Println("0 cinch:ignore declarations found")
		return 0
	}
	fmt.Printf("%d cinch:ignore declarations:\n\n", len(ignored))
	for _, item := range ignored {
		reason := item.ignoreReason
		if reason == "" {
			reason = "(no reason — malformed, see cinch check)"
		}
		fmt.Printf("%s %s: %s\n", item.id, item.file, reason)
	}
	return 0
}
