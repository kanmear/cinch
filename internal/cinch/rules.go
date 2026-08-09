package cinch

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ruleItemRe = regexp.MustCompile(`^\s*\d+\.\s+\*\*([A-Z0-9]+-[0-9]+)\*\*`)
	anyItemRe  = regexp.MustCompile(`^\s*\d+\.\s`)
	ignoreRe   = regexp.MustCompile(`<!--\s*cinch:ignore\s*(?::\s*(.*?))?\s*-->`)
	markerRe   = regexp.MustCompile(`//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)`)
)

// ruleItem is one rule: a numbered list item with a bolded ID
// (`1. **ID-NNN** ...`) somewhere under the docs dir.
type ruleItem struct {
	ID   string
	File string
	Line int // line of the ID

	// Text is the item-scoped rule text — the ID line through the last line
	// before the next numbered item — whitespace-normalized.
	Text string

	HasIgnore    bool
	IgnoreReason string
	IgnoreLine   int
}

type markerLoc struct {
	File string
	Line int
}

// parseRuleItems scans content line by line for rule items, stopping each
// item's text at the next numbered-list line (ID or not) or EOF. Exported as
// a function of (file, content) rather than just a path so the coupling
// check can parse both a working-tree file and a `git show HEAD:...` blob.
func parseRuleItems(file string, content []byte) []ruleItem {
	var items []ruleItem
	var cur *ruleItem
	var textLines []string
	inFence := false

	flush := func() {
		if cur == nil {
			return
		}
		cur.Text = strings.Join(strings.Fields(strings.Join(textLines, " ")), " ")
		items = append(items, *cur)
		cur = nil
		textLines = nil
	}

	lineNo := 0
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		if m := ruleItemRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &ruleItem{ID: m[1], File: file, Line: lineNo}
			textLines = []string{line}
			continue
		}
		if anyItemRe.MatchString(line) {
			flush()
			continue
		}
		if cur == nil {
			continue
		}
		textLines = append(textLines, line)
		if !cur.HasIgnore {
			if m := ignoreRe.FindStringSubmatch(line); m != nil {
				cur.HasIgnore = true
				cur.IgnoreReason = strings.TrimSpace(m[1])
				cur.IgnoreLine = lineNo
			}
		}
	}
	flush()

	return items
}

func parseRuleDoc(path string) []ruleItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseRuleItems(path, data)
}

func scanRuleDocs(root string) []ruleItem {
	var items []ruleItem
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		items = append(items, parseRuleDoc(path)...)
		return nil
	})
	return items
}

// scanRuleMarkers walks the whole repo (skipping .git, the docs dir, and the
// build output dir) for `// cinch:rule <ID>` comments.
func scanRuleMarkers(repoRoot, docsDir string) map[string][]markerLoc {
	markers := map[string][]markerLoc{}
	const maxSize = 4 << 20

	_ = filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin":
				return filepath.SkipDir
			}
			if path == docsDir {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lineNo := 0
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			lineNo++
			if m := markerRe.FindStringSubmatch(scanner.Text()); m != nil {
				markers[m[1]] = append(markers[m[1]], markerLoc{File: path, Line: lineNo})
			}
		}
		return nil
	})

	return markers
}

// checkRules fires on a rule ID with no marker and on a marker with no
// matching rule ID. Direction: lateral — a doc's authored rule IDs and
// source's authored markers must agree in both directions (principle 4's own
// worked example).
//
// cinch:ignore is a declaration, not a suppression (see standing rule 3 in
// .docs/cinch-rebuild-plan.md): a rule marked ignore with a reason is exempt
// from the missing-marker finding, but a rule that is both ignored and
// marked is a contradiction and is itself a finding, and an ignore with no
// reason is malformed and is itself a finding.
func checkRules(docsDir, repoRoot string) []Finding {
	items := scanRuleDocs(docsDir)
	markers := scanRuleMarkers(repoRoot, docsDir)

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

	ruleIDs := map[string]bool{}
	for _, item := range items {
		ruleIDs[item.ID] = true
	}
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

// CmdIgnores lists every cinch:ignore declaration under root with its
// reason. Not a check: it never fails and carries no findings — it exists so
// the ignore inventory is cheap to read periodically.
func CmdIgnores(docsDir string) int {
	items := scanRuleDocs(docsDir)
	for _, item := range items {
		if !item.HasIgnore {
			continue
		}
		reason := item.IgnoreReason
		if reason == "" {
			reason = "(no reason — malformed, see cinch check)"
		}
		fmt.Printf("%s %s: %s\n", item.ID, item.File, reason)
	}
	return 0
}
