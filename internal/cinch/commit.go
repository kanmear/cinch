package cinch

import (
	"os"
	"regexp"
	"strings"
)

// checkCommit validates msgFile's subject line (its first line) against the
// manifest's commit.pattern key, when both are present. No msgFile, no
// manifest, or no commit.pattern key is not an error — commit conventions
// are opt-in, and absence changes nothing.
func checkCommit(root, msgFile string) []Finding {
	if msgFile == "" {
		return nil
	}
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return nil
	}
	pattern, ok := m.Vars["commit.pattern"]
	if !ok || pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []Finding{{
			Check: "commit", Level: "error", File: msgFile, Line: 1,
			Message: "commit.pattern is not a valid regexp: " + err.Error(),
		}}
	}
	data, err := os.ReadFile(msgFile)
	if err != nil {
		return nil
	}
	subject, _, _ := strings.Cut(string(data), "\n")
	if re.MatchString(subject) {
		return nil
	}
	return []Finding{{
		Check: "commit", Level: "error", File: msgFile, Line: 1,
		Message: "commit message does not match commit.pattern (" + pattern + "): " + subject,
	}}
}
