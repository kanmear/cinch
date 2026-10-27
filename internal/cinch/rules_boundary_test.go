package cinch

import (
	"strings"
	"testing"
)

// The parser-level boundary cases live in rules_test.go, which paths.exclude
// keeps out of the marker scan; this is the end-to-end regression that
// carries the CINCH-014 binding.

// cinch:rule CINCH-014
func TestIgnoreOutsideRuleItemBindsNothing(t *testing.T) {
	doc := "# Auth\n\n" +
		"1. **AUTH-001** Sessions expire.\n" +
		"2. **AUTH-002** Tokens rotate.\n\n" +
		"## Notes\n\n" +
		"Example of the syntax:\n" +
		"<!-- cinch:ignore: just an example in prose -->\n"
	items := parseRuleItems("auth.md", []byte(doc))
	markers := map[string][]markerLoc{"AUTH-001": {{file: "auth_test.go", line: 3}}}

	report := checkRulesFrom(items, markers)
	if len(report.findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one for AUTH-002", report.findings)
	}
	f := report.findings[0]
	if f.line != 4 || !strings.HasPrefix(f.message, "AUTH-002:") {
		t.Fatalf("finding = %+v, want AUTH-002 at line 4 reported as unmarked", f)
	}
}
