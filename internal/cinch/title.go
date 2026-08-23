package cinch

import (
	"strconv"
	"strings"
)

func titleAndTrigger(body string) (title, trigger string) {
	lines := strings.Split(body, "\n")
	titleIdx := -1
	for i, l := range lines {
		if t, ok := strings.CutPrefix(l, "# "); ok {
			title = strings.TrimSpace(t)
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
			continue
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

func startsNewBlock(line string) bool {
	for _, prefix := range []string{"#", "-", "*", "```", "|", ">"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	if i := strings.IndexByte(line, '.'); i > 0 && i <= 2 {
		if _, err := strconv.Atoi(line[:i]); err == nil {
			return true
		}
	}
	return false
}

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
