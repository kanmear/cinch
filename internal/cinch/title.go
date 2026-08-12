package cinch

import "strconv"
import "strings"

// titleAndTrigger extracts a doc's H1 title and its trigger (the first
// sentence after it — when to read the file), both verbatim. Some docs
// hand-wrap that opening sentence across physical lines, so the trigger
// joins continuation lines until one ends in sentence punctuation, a blank
// line, or a new block (bullet, heading, fence, table, quote) — whichever
// comes first.
func titleAndTrigger(body string) (title, trigger string) {
	lines := strings.Split(body, "\n")
	titleIdx := -1
	for i, l := range lines {
		if t, ok := strings.CutPrefix(l, "# "); ok {
			title = t
			titleIdx = i
			break
		}
	}
	if titleIdx == -1 {
		return "", ""
	}

	var parts []string
	for _, l := range lines[titleIdx+1:] {
		line := strings.TrimSpace(l)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue // blank lines before the trigger starts (e.g. under the H1)
		}
		if len(parts) > 0 && startsNewBlock(line) {
			break
		}
		parts = append(parts, line)
		if endsSentence(line) {
			break
		}
	}
	trigger = strings.Join(parts, " ")
	return title, trigger
}

// startsNewBlock reports whether line opens a new markdown block (heading,
// list item, fence, table row, blockquote) rather than continuing prose.
func startsNewBlock(line string) bool {
	for _, prefix := range []string{"#", "-", "*", "```", "|", ">"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	if i := strings.IndexByte(line, '.'); i > 0 && i <= 2 {
		if _, err := strconv.Atoi(line[:i]); err == nil {
			return true // ordered list item, e.g. "1. "
		}
	}
	return false
}

// endsSentence reports whether line's last character terminates a sentence.
func endsSentence(line string) bool {
	if line == "" {
		return false
	}
	switch line[len(line)-1] {
	case '.', '!', '?':
		return true
	}
	return false
}
