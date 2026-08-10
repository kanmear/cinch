package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func renderToScratch(t *testing.T, root string) {
	t.Helper()
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

	target := filepath.Join(root, ".docs", "workflows", "maintain-domain.md")
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
	if got[0].File != ".docs/workflows/maintain-domain.md" {
		t.Fatalf("unexpected finding: %+v", got[0])
	}
}

func TestCheckGenerated_DeletedFileFires(t *testing.T) {
	root := t.TempDir()
	renderToScratch(t, root)

	target := filepath.Join(root, ".docs", "workflows", "fix-bug.md")
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove %s: %v", target, err)
	}

	result := checkGenerated(root)

	got := findingsForCheck(result.Findings, "generated")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].File != ".docs/workflows/fix-bug.md" {
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

	// Re-render so the doc map catches up with the orphan's presence on
	// disk (buildDocMap's on-disk union sees it as content, same as any
	// other file it doesn't know is orphaned) — isolating the orphan
	// finding from an incidental "doc map is stale" finding.
	renderToScratch(t, root)

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
	renderToScratch(t, root) // catch the doc map up with notes.md's presence

	result := checkGenerated(root)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings for a non-generated file, got %+v", result.Findings)
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
