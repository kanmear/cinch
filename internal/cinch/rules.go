package cinch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"cinch/internal/concurrency"
	"cinch/internal/gitutil"
	"cinch/internal/mdscan"
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

// levenshtein returns the edit distance between a and b (insertions,
// deletions, substitutions each cost 1).
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

// nearMissRuleIDs returns up to 2 candidate IDs from ruleIDs that are
// plausible typos of id (edit distance <= 2), sorted by (distance, id) for
// determinism across runs despite ruleIDs being an unordered map. Callers
// only invoke this on an id that is not itself present in ruleIDs.
func nearMissRuleIDs(id string, ruleIDs map[string]bool) []string {
	type candidate struct {
		id       string
		distance int
	}
	var candidates []candidate
	for other := range ruleIDs {
		if d := levenshtein(id, other); d <= 2 {
			candidates = append(candidates, candidate{id: other, distance: d})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].id < candidates[j].id
	})
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]string, len(candidates))
	for i, c := range candidates {
		out[i] = c.id
	}
	return out
}

// duplicateLocsMessage describes every location that declared the same rule
// ID, for a "declared in N places" finding. locs must have at least 2
// elements.
func duplicateLocsMessage(locs []ruleItem) string {
	pairs := make([]string, len(locs))
	for i, loc := range locs {
		pairs[i] = fmt.Sprintf("%s:%d", loc.file, loc.line)
	}
	if len(pairs) == 2 {
		return "declared in two places (" + pairs[0] + " and " + pairs[1] + ") — rule IDs must be unique"
	}
	return fmt.Sprintf("declared in %d places (%s) — rule IDs must be unique", len(pairs), strings.Join(pairs, ", "))
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

	_ = mdscan.ForEachFencedLineBytes(content, func(lineNumber int, line string) {
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
	return mdscan.Collect(docsRoot, parseRuleDoc)
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
	if files, err := gitutil.ScannableFiles(repoRoot); err == nil {
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
		err = mdscan.ForEachFencedLine(r, fn)
	} else {
		err = mdscan.ForEachLine(r, fn)
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
	for i := 0; i < concurrency.BoundedWorkers(len(files)); i++ {
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
		message := "// cinch:rule " + id + " does not resolve to any rule ID"
		if suggestions := nearMissRuleIDs(id, ruleIDs); len(suggestions) > 0 {
			message += " (did you mean " + strings.Join(suggestions, " or ") + "?)"
		}
		for _, loc := range locs {
			findings = append(findings, finding{
				check: "rules", level: "error", file: loc.file, line: loc.line,
				message: message,
			})
		}
	}

	docs := make(map[string]bool, len(items))
	byID := make(map[string][]ruleItem, len(items))
	ignores := 0
	for _, item := range items {
		docs[item.file] = true
		byID[item.id] = append(byID[item.id], item)
		if item.hasIgnore {
			ignores++
		}
	}

	dupIDs := make([]string, 0, len(byID))
	for id := range byID {
		dupIDs = append(dupIDs, id)
	}
	sort.Strings(dupIDs)
	for _, id := range dupIDs {
		locs := byID[id]
		if len(locs) <= 1 {
			continue
		}
		findings = append(findings, finding{
			check: "rules", level: "error", file: locs[0].file, line: locs[0].line,
			message: id + ": " + duplicateLocsMessage(locs),
		})
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
