package main

import (
	"path/filepath"
	"testing"
)

func TestRules_MissingMarkerFires(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "rules.md"), "1. **CIN-001** the rule text.\n")

	findings := checkRules(docs, root)

	got := findingsForCheck(findings, "rules")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].Line != 1 {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestRules_PresentMarkerIsClean(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "rules.md"), "1. **CIN-001** the rule text.\n")
	writeFile(t, filepath.Join(root, "foo_test.go"), "// cinch:rule CIN-001\nfunc TestFoo(t *testing.T) {}\n")

	findings := checkRules(docs, root)

	if got := findingsForCheck(findings, "rules"); len(got) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(got), got)
	}
}

func TestRules_DanglingMarkerFires(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(root, "foo_test.go"), "// cinch:rule CIN-999\nfunc TestFoo(t *testing.T) {}\n")

	findings := checkRules(docs, root)

	got := findingsForCheck(findings, "rules")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].File != filepath.Join(root, "foo_test.go") || got[0].Line != 1 {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestRuleRegex_RejectsNumberedProse(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "plan.md"),
		"1. **Pure function of the tree plus HEAD.** No stored state, no cache.\n"+
			"2. **Every check ships with the mutation that proves it catches something.**\n")

	items := scanRuleDocs(docs)

	if len(items) != 0 {
		t.Fatalf("want 0 rule items parsed from numbered prose, got %d: %+v", len(items), items)
	}
}

func TestRules_IgnoreWithReasonIsClean(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "rules.md"),
		"1. **CIN-002** asserts the absence of a flow.\n"+
			"   <!-- cinch:ignore: asserts absence of a flow, no test to point at -->\n")

	findings := checkRules(docs, root)

	if got := findingsForCheck(findings, "rules"); len(got) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(got), got)
	}
}

func TestRules_IgnoreWithoutReasonFires(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "rules.md"),
		"1. **CIN-002** asserts the absence of a flow.\n"+
			"   <!-- cinch:ignore -->\n")

	findings := checkRules(docs, root)

	got := findingsForCheck(findings, "rules")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Fatalf("want finding on the ignore line, got: %+v", got[0])
	}
}

func TestRules_IgnoreAndMarkerBothPresentFires(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "rules.md"),
		"1. **CIN-002** asserts the absence of a flow.\n"+
			"   <!-- cinch:ignore: asserts absence of a flow -->\n")
	writeFile(t, filepath.Join(root, "foo_test.go"), "// cinch:rule CIN-002\nfunc TestFoo(t *testing.T) {}\n")

	findings := checkRules(docs, root)

	got := findingsForCheck(findings, "rules")
	if len(got) != 1 {
		t.Fatalf("want 1 contradiction finding, got %d: %+v", len(got), got)
	}
}
