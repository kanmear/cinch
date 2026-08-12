package cinch

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexList_ListsAuthoredDocsAndRenderedWorkflows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docs", "notes.md"), "# Hand-authored Notes\n\nSome content.\n")

	m := &Manifest{Vars: map[string]string{"paths.docs": ".docs"}}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		writeFile(t, filepath.Join(root, f.Dest), f.Body)
	}

	list, err := indexList(filepath.Join(root, ".docs"))
	if err != nil {
		t.Fatalf("indexList: %v", err)
	}
	if !strings.Contains(list, "notes.md — Hand-authored Notes") {
		t.Fatalf("indexList: missing hand-authored doc entry:\n%s", list)
	}
	if !strings.Contains(list, "workflows/docs-maintain-domain.md") {
		t.Fatalf("indexList: missing rendered workflow entry:\n%s", list)
	}
}

func TestIndexList_ExcludesPlansDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docs", "plans", "scratch.md"), "# Scratch Plan\n\nWork in progress.\n")
	writeFile(t, filepath.Join(root, ".docs", "notes.md"), "# Notes\n\nContent.\n")

	list, err := indexList(filepath.Join(root, ".docs"))
	if err != nil {
		t.Fatalf("indexList: %v", err)
	}
	if strings.Contains(list, "scratch") {
		t.Fatalf("indexList: plans/ entry leaked in:\n%s", list)
	}
}

func TestIndexList_SkipsDocsWithNoH1(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docs", "broken.md"), "no title here\n")
	writeFile(t, filepath.Join(root, ".docs", "notes.md"), "# Notes\n\nContent.\n")

	list, err := indexList(filepath.Join(root, ".docs"))
	if err != nil {
		t.Fatalf("indexList: %v", err)
	}
	if strings.Contains(list, "broken.md") {
		t.Fatalf("indexList: untitled doc should be skipped, not listed:\n%s", list)
	}
	if !strings.Contains(list, "notes.md") {
		t.Fatalf("indexList: titled doc missing:\n%s", list)
	}
}

func TestIndexList_EmptyCorpusFires(t *testing.T) {
	root := t.TempDir()
	if _, err := indexList(filepath.Join(root, ".docs")); err == nil {
		t.Fatalf("indexList: want an error for an empty/absent docs root, got nil")
	}
}

func TestCmdIndex_RenderNotRunFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")

	if code := CmdIndex(root); code != 1 {
		t.Fatalf("CmdIndex: want exit 1 when there's nothing to show, got %d", code)
	}
}

func TestCmdIndex_PrintsDocList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")
	writeFile(t, filepath.Join(root, ".docs", "notes.md"), "# Notes\n\nContent.\n")

	if code := CmdIndex(root); code != 0 {
		t.Fatalf("CmdIndex: want exit 0, got %d", code)
	}
}
