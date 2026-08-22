package cinch

import (
	"os"
	"regexp"
	"strings"
)

func checkCommit(root, messageFile string) checkResult {
	if messageFile == "" {
		return checkResult{noOp: "no commit message file given"}
	}
	pattern, ok, _ := manifestSetting(root, "commit.pattern")
	if !ok {
		return checkResult{noOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "commit", level: "error", file: messageFile, line: 1,
			message: "commit.pattern is not a valid regexp: " + err.Error(),
		}}}
	}
	data, err := os.ReadFile(messageFile)
	if err != nil {
		return checkResult{noOp: "could not read " + messageFile}
	}
	subject, _, _ := strings.Cut(string(data), "\n")
	if re.MatchString(subject) {
		return checkResult{}
	}
	return checkResult{findings: []finding{{
		check: "commit", level: "error", file: messageFile, line: 1,
		message: "commit message does not match commit.pattern (" + pattern + "): " + subject,
	}}}
}
