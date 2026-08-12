package cinch

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestListWorkflowNames_SortsAlphabetically(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Vars: map[string]string{"paths.docs": ".docs"}}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), f.Body)
	}

	names, err := listWorkflowNames(filepath.Join(root, ".docs"))
	if err != nil {
		t.Fatalf("listWorkflowNames: %v", err)
	}
	if !sortedStrings(names) {
		t.Fatalf("listWorkflowNames: want sorted, got %v", names)
	}
	if len(names) == 0 {
		t.Fatalf("listWorkflowNames: want at least one workflow, got none")
	}
}

func sortedStrings(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

func TestCmdWorkflows_RenderNotRunFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")

	if code := CmdWorkflows(root); code != 1 {
		t.Fatalf("CmdWorkflows: want exit 1 when render hasn't run, got %d", code)
	}
}

func TestCmdWorkflows_PrintsTriggerTable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), header(f.Source, f.Body, f.Style)+f.Body)
	}

	if code := CmdWorkflows(root); code != 0 {
		t.Fatalf("CmdWorkflows: want exit 0, got %d", code)
	}
}

// TestWorkflowsTable_ListsEveryWorkflow is the on-demand equivalent of the
// old generated-index coverage check: every rendered workflow must appear
// as a row in the computed table.
func TestWorkflowsTable_ListsEveryWorkflow(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Vars: map[string]string{"paths.docs": ".docs"}}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), f.Body)
	}

	table, err := workflowsTable(filepath.Join(root, ".docs"))
	if err != nil {
		t.Fatalf("workflowsTable: %v", err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Dest, ".docs/"+workflowsSubdir+"/") {
			continue
		}
		name := filepath.Base(f.Dest)
		if !strings.Contains(table, name) {
			t.Fatalf("workflowsTable: missing entry for %s:\n%s", name, table)
		}
	}
}

func TestCmdWorkflow_CustomDocsRootFromManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: mydocs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), header(f.Source, f.Body, f.Style)+f.Body)
	}

	if code := CmdWorkflow(root, "docs-maintain-domain"); code != 0 {
		t.Fatalf("CmdWorkflow: want exit 0 resolving under a custom paths.docs, got %d", code)
	}
}

func TestCmdWorkflow_UnknownNameFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), header(f.Source, f.Body, f.Style)+f.Body)
	}

	if code := CmdWorkflow(root, "bogus-name"); code != 1 {
		t.Fatalf("CmdWorkflow: want exit 1 when render hasn't run, got %d", code)
	}
}

func TestCmdWorkflow_RenderNotRunFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")

	if code := CmdWorkflow(root, "docs-maintain-domain"); code != 1 {
		t.Fatalf("CmdWorkflow: want exit 1 when render hasn't run, got %d", code)
	}
}
