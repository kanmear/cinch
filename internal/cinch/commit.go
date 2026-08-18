package cinch

import (
	"os"
	"regexp"
	"strings"
)

type commitResult struct {
	Findings []Finding
	NoOp     string
}

func checkCommit(root, msgFile string) commitResult {
	if msgFile == "" {
		return commitResult{NoOp: "no commit message file given"}
	}
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return commitResult{NoOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	pattern, ok := m.Vars["commit.pattern"]
	if !ok || pattern == "" {
		return commitResult{NoOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return commitResult{Findings: []Finding{{
			Check: "commit", Level: "error", File: msgFile, Line: 1,
			Message: "commit.pattern is not a valid regexp: " + err.Error(),
		}}}
	}
	data, err := os.ReadFile(msgFile)
	if err != nil {
		return commitResult{NoOp: "could not read " + msgFile}
	}
	subject, _, _ := strings.Cut(string(data), "\n")
	if re.MatchString(subject) {
		return commitResult{}
	}
	return commitResult{Findings: []Finding{{
		Check: "commit", Level: "error", File: msgFile, Line: 1,
		Message: "commit message does not match commit.pattern (" + pattern + "): " + subject,
	}}}
}
