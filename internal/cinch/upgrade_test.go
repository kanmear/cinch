package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdUpgradeRendersSyncsAndChecksClean(t *testing.T) {
	withVersion(t, "1.2.4")
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: 1.2.3\n")

	// simulate a stale previously-rendered workflow file, as a prior cinch
	// version would have left it, so the overwrite path is exercised.
	staleDirectory := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	if err := os.MkdirAll(staleDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, staleDirectory, "docs-philosophy.md", "stale content\n")

	if code := CmdUpgrade(root); code != 0 {
		t.Fatalf("CmdUpgrade = %d, want 0", code)
	}

	m := loadTestManifest(t, root)
	if got, ok := manifestSetting(m, requireCinchKey); !ok || got != "1.2.4" {
		t.Fatalf("require.cinch = %q, ok=%v, want 1.2.4", got, ok)
	}
	if _, err := os.Stat(filepath.Join(root, defaultHooksPath, "pre-commit")); err != nil {
		t.Fatalf("expected hook shim to be rendered: %v", err)
	}

	// audit's own verification wording: core, generated, and rules must be
	// clean immediately after CmdUpgrade, in the same invocation.
	if code := runChecks(workingTreeRoots(root), "", false, "core", "generated", "rules"); code != 0 {
		t.Fatalf("post-upgrade checks = %d, want 0 (clean)", code)
	}
}

// A template a newer cinch no longer ships leaves its rendered file behind;
// upgrade has to remove it or its own closing check fails.
func TestCmdUpgradeRemovesDroppedTemplate(t *testing.T) {
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "")
	directory := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	droppedBody := "# Dropped workflow\n"
	writeTestFile(t, directory, "dropped.md", header("docs/templates/dropped.md", droppedBody, styleMarkdown)+droppedBody)

	if code := CmdUpgrade(root); code != 0 {
		t.Fatalf("CmdUpgrade = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(directory, "dropped.md")); !os.IsNotExist(err) {
		t.Fatalf("dropped.md stat err = %v, want it removed", err)
	}
	if code := runChecks(workingTreeRoots(root), "", false, "generated"); code != 0 {
		t.Fatalf("generated check = %d, want 0 (clean)", code)
	}
}

// cinch:rule CINCH-008
func TestCmdUpgradeRangePinSatisfiedLeavesPinUnchanged(t *testing.T) {
	withVersion(t, "1.2.4")
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
	// already rendered: this upgrade produces the same bytes
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}

	if code := CmdUpgrade(root); code != 0 {
		t.Fatalf("CmdUpgrade = %d, want 0", code)
	}

	m := loadTestManifest(t, root)
	if got, ok := manifestSetting(m, requireCinchKey); !ok || got != ">=1.0.0" {
		t.Fatalf("require.cinch = %q, ok=%v, want unchanged >=1.0.0", got, ok)
	}
}

func TestCmdUpgradeRangePinUnsatisfiedSurfacesCheckFailure(t *testing.T) {
	withVersion(t, "1.0.0")
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '>=2.0.0'\n")

	if code := CmdUpgrade(root); code == 0 {
		t.Fatalf("CmdUpgrade = %d, want nonzero (installed version does not satisfy range pin)", code)
	}
}

// cinch:rule CINCH-008
func TestCmdUpgradeRaisesFloorWhenOutputChanged(t *testing.T) {
	withVersion(t, "1.2.4")
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	// stand in for an older cinch's output
	writeTestFile(t, filepath.Join(root, defaultHooksPath), "pre-commit", "#!/bin/sh\nold\n")

	if code := CmdUpgrade(root); code != 0 {
		t.Fatalf("CmdUpgrade = %d, want 0", code)
	}

	m := loadTestManifest(t, root)
	if got, ok := manifestSetting(m, requireCinchKey); !ok || got != ">=1.2.4" {
		t.Fatalf("require.cinch = %q, ok=%v, want raised to >=1.2.4", got, ok)
	}
}
