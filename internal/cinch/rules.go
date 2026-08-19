package cinch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cinch/internal/output"
)

var (
	ruleItemRe = regexp.MustCompile(`^\s*\d+\.\s+\*\*([A-Z0-9]+-[0-9]+)\*\*`)
	anyItemRe  = regexp.MustCompile(`^\s*\d+\.\s`)
	ignoreRe   = regexp.MustCompile(`<!--\s*cinch:ignore\s*(?::\s*(.*?))?\s*-->`)
	markerRe   = regexp.MustCompile(`//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)\b`)
)

type ruleItem struct {
	ID   string
	File string
	Line int

	Text string

	HasIgnore    bool
	IgnoreReason string
	IgnoreLine   int
}

type markerLoc struct {
	File string
	Line int
}

func ruleItemIDs(items []ruleItem) map[string]bool {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.ID] = true
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
		cur.Text = strings.Join(strings.Fields(strings.Join(textLines, " ")), " ")
		items = append(items, *cur)
		cur = nil
		textLines = nil
	}

	_ = forEachFencedLine(content, func(lineNo int, line string) {
		if m := ruleItemRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &ruleItem{ID: m[1], File: file, Line: lineNo}
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
		if !cur.HasIgnore {
			if m := ignoreRe.FindStringSubmatch(line); m != nil {
				cur.HasIgnore = true
				cur.IgnoreReason = strings.TrimSpace(m[1])
				cur.IgnoreLine = lineNo
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

func scanRuleDocs(root string) ([]ruleItem, error) {
	return collectMarkdown(root, parseRuleDoc)
}

func underPath(path, dir string) bool {
	if dir == "" {
		return false
	}
	absPath, err1 := filepath.Abs(path)
	absDir, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func markerScanFiles(root, docsDir string) ([]string, error) {
	if files, err := gitScannableFiles(root); err == nil {
		var out []string
		for _, f := range files {
			if !underPath(f, docsDir) {
				out = append(out, f)
			}
		}
		return out, nil
	}

	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || underPath(path, docsDir) {
				return filepath.SkipDir
			}
			return nil
		}
		if underPath(path, docsDir) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
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

func scanMarkers(r io.Reader, path string, fenced bool, markers map[string][]markerLoc) error {
	fn := func(lineNo int, line string) {
		if m := markerRe.FindStringSubmatch(line); m != nil {
			markers[m[1]] = append(markers[m[1]], markerLoc{File: path, Line: lineNo})
		}
	}
	if fenced {
		return forEachFencedLineReader(r, fn)
	}
	return forEachLineReader(r, fn)
}

func scanFileMarkers(path string, markers map[string][]markerLoc) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return scanMarkers(f, path, strings.HasSuffix(path, ".md"), markers)
}

func scanRuleMarkers(repoRoot, docsDir string) (map[string][]markerLoc, error) {
	markers := map[string][]markerLoc{}

	files, err := markerScanFiles(repoRoot, docsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		path := filepath.Join(repoRoot, file)
		if err := scanFileMarkers(path, markers); err != nil {
			return nil, err
		}
	}

	return markers, nil
}

func checkRules(docsDir, repoRoot string) []Finding {
	items, err := scanRuleDocs(docsDir)
	if err != nil {
		return scanErrorFinding("rules", docsDir, err)
	}
	markers, err := scanRuleMarkers(repoRoot, docsDir)
	if err != nil {
		return scanErrorFinding("rules", repoRoot, err)
	}

	return checkRulesFrom(items, markers)
}

func checkRulesFrom(items []ruleItem, markers map[string][]markerLoc) []Finding {
	var findings []Finding

	for _, item := range items {
		_, marked := markers[item.ID]

		if item.HasIgnore && item.IgnoreReason == "" {
			findings = append(findings, Finding{
				Check: "rules", Level: "error", File: item.File, Line: item.IgnoreLine,
				Message: item.ID + ": cinch:ignore has no reason",
			})
		}
		if item.HasIgnore && item.IgnoreReason != "" && marked {
			findings = append(findings, Finding{
				Check: "rules", Level: "error", File: item.File, Line: item.Line,
				Message: item.ID + ": declared cinch:ignore but also has a // cinch:rule marker — pick one",
			})
		}
		if !item.HasIgnore && !marked {
			findings = append(findings, Finding{
				Check: "rules", Level: "error", File: item.File, Line: item.Line,
				Message: item.ID + ": no // cinch:rule marker (or cinch:ignore declaration)",
			})
		}
	}

	ruleIDs := ruleItemIDs(items)
	for id, locs := range markers {
		if ruleIDs[id] {
			continue
		}
		for _, loc := range locs {
			findings = append(findings, Finding{
				Check: "rules", Level: "error", File: loc.File, Line: loc.Line,
				Message: "// cinch:rule " + id + " does not resolve to any rule ID",
			})
		}
	}

	return findings
}

func CmdIgnores(docsDir string) int {
	items, err := scanRuleDocs(docsDir)
	if err != nil {
		return output.Fail("ignores", err)
	}
	var ignored []ruleItem
	for _, item := range items {
		if item.HasIgnore {
			ignored = append(ignored, item)
		}
	}
	if len(ignored) == 0 {
		fmt.Println("0 cinch:ignore declarations found")
		return 0
	}
	fmt.Printf("%d cinch:ignore declarations:\n\n", len(ignored))
	for _, item := range ignored {
		reason := item.IgnoreReason
		if reason == "" {
			reason = "(no reason — malformed, see cinch check)"
		}
		fmt.Printf("%s %s: %s\n", item.ID, item.File, reason)
	}
	return 0
}
