package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderToScratch(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	files, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		dst := filepath.Join(root, f.Dest)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		content := header(f.Source, f.Body, f.Style) + f.Body
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dst, []byte(content), mode); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
}

func TestCheckGenerated_FreshRenderIsClean(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	result := checkGenerated(root)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp != "" {
		t.Fatalf("want a real comparison, got no-op: %s", result.NoOp)
	}
}

func TestCheckGenerated_ByteFlipFires(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	target := filepath.Join(root, ".docs", "workflows", "docs-maintain-domain.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if err := os.WriteFile(target, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}

	result := checkGenerated(root)

	got := findingsForCheck(result.Findings, "generated")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].File != ".docs/workflows/docs-maintain-domain.md" {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestCheckGenerated_DeletedFileFires(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	target := filepath.Join(root, ".docs", "workflows", "dev-fix-bug.md")
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove %s: %v", target, err)
	}

	result := checkGenerated(root)

	got := findingsForCheck(result.Findings, "generated")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].File != ".docs/workflows/dev-fix-bug.md" {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestCheckGenerated_OrphanFires(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	// Simulate a template that existed at a previous render and was since
	// removed from cinch: its old output is still on disk, tagged with a
	// real generated header, but no longer in this render pass's file set.
	orphan := filepath.Join(root, ".docs", "workflows", "stale-workflow.md")
	writeFile(t, orphan, headerPrefix+" from docs/templates/stale-workflow.md — do not edit; body-sha: deadbeef -->\n\n# Stale\n\nNo longer produced.\n")

	result := checkGenerated(root)

	got := findingsForCheck(result.Findings, "generated")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].File != ".docs/workflows/stale-workflow.md" {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestCheckGenerated_HandAuthoredOrphanIsIgnored(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	writeFile(t, filepath.Join(root, ".docs", "workflows", "notes.md"), "# Not Generated\n\nA hand-authored file dropped in the workflows dir.\n")

	result := checkGenerated(root)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings for a non-generated file, got %+v", result.Findings)
	}
}

// TestCheckGenerated_DeletedWorkflowsDirStillVerifiesShims is the mutation
// fixture for the missing-directory gate: verification used to be skipped
// entirely when the workflows dir was absent, which also skipped the outputs
// that live outside the docs root. Deleting one directory was therefore
// enough to edit a generated hook shim without `cinch check` failing — the
// exact claim the README makes about tamper-evidence.
func TestCheckGenerated_DeletedWorkflowsDirStillVerifiesShims(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	if err := os.RemoveAll(filepath.Join(root, ".docs", "workflows")); err != nil {
		t.Fatalf("remove workflows dir: %v", err)
	}
	shim := filepath.Join(root, ".githooks", "pre-commit")
	writeFile(t, shim, "#!/bin/sh\nexit 0\n")

	result := checkGenerated(root)

	if result.NoOp != "" {
		t.Fatalf("a rendered tree missing its workflows dir is not 'render has not run': %s", result.NoOp)
	}
	got := findingsForCheck(result.Findings, "generated")

	tampered := false
	missingWorkflow := false
	for _, f := range got {
		if f.File == ".githooks/pre-commit" {
			tampered = true
		}
		if strings.HasPrefix(f.File, ".docs/workflows/") {
			missingWorkflow = true
		}
	}
	if !tampered {
		t.Fatalf("want a finding for the tampered hook shim, got: %+v", got)
	}
	if !missingWorkflow {
		t.Fatalf("want findings for the deleted workflow files, got: %+v", got)
	}
}

func TestCheckGenerated_UnrenderedRepoAnnouncesNoOp(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch_manifest"), "paths.docs = .docs\n")

	result := checkGenerated(root)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message when render hasn't run, got none")
	}
}

func TestCheckGenerated_NoManifestAnnouncesNoOp(t *testing.T) {
	root := t.TempDir()

	result := checkGenerated(root)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message with no manifest, got none")
	}
}
