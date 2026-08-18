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
		inFence := false
		scanner := bufio.NewScanner(bytes.NewReader(data))
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
			if m := markerRe.FindStringSubmatch(line); m != nil {
				markers[m[1]] = append(markers[m[1]], markerLoc{File: path, Line: lineNo})
			}
		}
		return nil
	})

	return markers
}

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

func CmdIgnores(docsDir string) int {
	items := scanRuleDocs(docsDir)
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
