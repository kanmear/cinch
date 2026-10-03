package cinch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRulesInventoryRoundTrip(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "sig.md", "---\nowns:\n  - internal/sig\nrule_prefix: SIG-\n---\n"+
		"# Signatures\n\n1. **SIG-1** verify the signature\n2. **SIG-2** reject expired signatures\n")

	src := filepath.Join(root, "internal", "sig")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, "verify.go", "package sig\n\n// cinch:rule SIG-1\nfunc Verify() {}\n")

	inv, err := buildRulesInventory(root, docs, nil)
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got rulesInventoryJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(got.Rules))
	}
	if len(got.Docs) != 1 || got.Docs[0].RulePrefix != "SIG-" {
		t.Fatalf("Docs = %+v, want one entry with RulePrefix SIG-", got.Docs)
	}
}

func TestBuildRulesInventoryDuplicateIDsIncluded(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "1. **DUP-1** first declaration\n")
	writeTestFile(t, docs, "b.md", "1. **DUP-1** second declaration\n")

	inv, err := buildRulesInventory(root, docs, nil)
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}
	if len(inv.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2 (duplicates included, not deduped)", len(inv.Rules))
	}
}

func TestBuildRulesInventoryDocsExcludesFrontmatterless(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "plain.md", "1. **PLAIN-1** no frontmatter here\n")

	inv, err := buildRulesInventory(root, docs, nil)
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}
	if len(inv.Docs) != 0 {
		t.Fatalf("Docs = %+v, want empty (no doc declares owns:/rule_prefix:)", inv.Docs)
	}
	if len(inv.Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(inv.Rules))
	}
}

func TestBuildRulesInventoryMarkersAttached(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "1. **RUL-1** has a marker\n")

	src := filepath.Join(root, "internal")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, "impl.go", "package internal\n\n// cinch:rule RUL-1\nfunc F() {}\n")

	inv, err := buildRulesInventory(root, docs, nil)
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}
	if len(inv.Rules) != 1 || len(inv.Rules[0].Markers) != 1 {
		t.Fatalf("Rules = %+v, want one rule with one marker", inv.Rules)
	}
	if inv.Rules[0].Markers[0].Line != 3 {
		t.Fatalf("marker line = %d, want 3", inv.Rules[0].Markers[0].Line)
	}
}

func TestBuildRulesInventoryCountMatchesCheckDetail(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "1. **CNT-1** first\n2. **CNT-2** second\n")

	inv, err := buildRulesInventory(root, docs, nil)
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}
	report := checkRules(workingTreeRoots(root), docs, nil)
	if len(inv.Rules) != report.rules {
		t.Fatalf("len(inv.Rules) = %d, checkRules.rules = %d, want equal", len(inv.Rules), report.rules)
	}
}
