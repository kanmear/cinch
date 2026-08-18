package cinch

import (
	"os"
	"regexp"
	"strings"
)

func checkCommit(root, msgFile string) checkResult {
	if msgFile == "" {
		return checkResult{NoOp: "no commit message file given"}
	}
	pattern, ok, _ := manifestSetting(root, "commit.pattern")
	if !ok {
		return checkResult{NoOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return checkResult{Findings: []Finding{{
			Check: "commit", Level: "error", File: msgFile, Line: 1,
			Message: "commit.pattern is not a valid regexp: " + err.Error(),
		}}}
	}
	data, err := os.ReadFile(msgFile)
	if err != nil {
		return checkResult{NoOp: "could not read " + msgFile}
	}
	subject, _, _ := strings.Cut(string(data), "\n")
	if re.MatchString(subject) {
		return checkResult{}
	}
	return checkResult{Findings: []Finding{{
		Check: "commit", Level: "error", File: msgFile, Line: 1,
		Message: "commit message does not match commit.pattern (" + pattern + "): " + subject,
	}}}
}
