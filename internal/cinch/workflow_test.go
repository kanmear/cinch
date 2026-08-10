package cinch

import (
	"path/filepath"
	"testing"
)

func TestListWorkflowNames_ExcludesIndexSortsAlphabetically(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Vars: map[string]string{"paths.docs": ".docs"}}
	files, err := renderAll(m, root)
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
	for _, n := range names {
		if n == "index" {
			t.Fatalf("listWorkflowNames: index.md leaked into the name list: %v", names)
		}
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
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")

	if code := CmdWorkflows(root); code != 1 {
		t.Fatalf("CmdWorkflows: want exit 1 when render hasn't run, got %d", code)
	}
}

func TestCmdWorkflows_PrintsTriggerTable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m, root)
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

func TestCmdWorkflow_CustomDocsRootFromManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = mydocs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m, root)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), header(f.Source, f.Body, f.Style)+f.Body)
	}

	if code := CmdWorkflow(root, "maintain-domain"); code != 0 {
		t.Fatalf("CmdWorkflow: want exit 0 resolving under a custom paths.docs, got %d", code)
	}
}

func TestCmdWorkflow_UnknownNameFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m, root)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), header(f.Source, f.Body, f.Style)+f.Body)
	}

	if code := CmdWorkflow(root, "bogus-name"); code != 1 {
		t.Fatalf("CmdWorkflow: want exit 1 for an unknown name, got %d", code)
	}
}

func TestCmdWorkflow_RenderNotRunFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")

	if code := CmdWorkflow(root, "maintain-domain"); code != 1 {
		t.Fatalf("CmdWorkflow: want exit 1 when render hasn't run, got %d", code)
	}
}
