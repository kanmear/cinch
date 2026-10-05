package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

// withVersion sets the package-level Version for the duration of the test
// and restores it afterward.
func withVersion(t *testing.T, version string) {
	t.Helper()
	old := Version
	Version = version
	t.Cleanup(func() { Version = old })
}

func readTestFile(t *testing.T, directory, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSyncRequireCinchNotConfigured(t *testing.T) {
	withVersion(t, "1.2.3")
	directory := t.TempDir()
	content := "paths:\n  docs: .docs\n"
	writeTestFile(t, directory, "cinch.yml", content)

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	if got := readTestFile(t, directory, "cinch.yml"); got != content {
		t.Fatalf("cinch.yml = %q, want unchanged %q", got, content)
	}
}

func TestSyncRequireCinchDevBuildSkips(t *testing.T) {
	withVersion(t, "dev")
	directory := t.TempDir()
	content := "require:\n  cinch: 1.2.3\n"
	writeTestFile(t, directory, "cinch.yml", content)

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	if got := readTestFile(t, directory, "cinch.yml"); got != content {
		t.Fatalf("cinch.yml = %q, want unchanged %q", got, content)
	}
}

func TestSyncRequireCinchAlreadyMatches(t *testing.T) {
	withVersion(t, "1.2.3")
	directory := t.TempDir()
	content := "require:\n  cinch: 1.2.3\n"
	writeTestFile(t, directory, "cinch.yml", content)

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	if got := readTestFile(t, directory, "cinch.yml"); got != content {
		t.Fatalf("cinch.yml = %q, want unchanged %q", got, content)
	}
}

// cinch:rule CINCH-004
func TestSyncRequireCinchRewritesExactMismatch(t *testing.T) {
	withVersion(t, "1.2.4")
	directory := t.TempDir()
	original := "paths:\n  docs: .docs\n  hooks: .githooks\n\n# pin the tool version\nrequire:\n  cinch: 1.2.3\n"
	writeTestFile(t, directory, "cinch.yml", original)

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	want := "paths:\n  docs: .docs\n  hooks: .githooks\n\n# pin the tool version\nrequire:\n  cinch: 1.2.4\n"
	if got := readTestFile(t, directory, "cinch.yml"); got != want {
		t.Fatalf("cinch.yml = %q, want %q (only the pin value should change)", got, want)
	}
}

func TestSyncRequireCinchPreservesQuoteStyle(t *testing.T) {
	withVersion(t, "1.2.4")
	directory := t.TempDir()
	writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '1.2.3'  # keep me quoted\n")

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	want := "require:\n  cinch: '1.2.4'  # keep me quoted\n"
	if got := readTestFile(t, directory, "cinch.yml"); got != want {
		t.Fatalf("cinch.yml = %q, want %q", got, want)
	}
}

// cinch:rule CINCH-004
func TestSyncRequireCinchLeavesRangeUntouched(t *testing.T) {
	withVersion(t, "1.9.9")
	directory := t.TempDir()
	content := "require:\n  cinch: '>=2.0.0'\n"
	writeTestFile(t, directory, "cinch.yml", content)

	if err := syncRequireCinch(directory); err != nil {
		t.Fatalf("syncRequireCinch: %v", err)
	}

	m := loadTestManifest(t, directory)
	if got, ok := manifestSetting(m, requireCinchKey); !ok || got != ">=2.0.0" {
		t.Fatalf("require.cinch = %q, ok=%v, want unchanged >=2.0.0", got, ok)
	}
}

func TestCmdRenderSyncsRequireCinch(t *testing.T) {
	withVersion(t, "1.2.4")
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: 1.2.3\n")

	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}

	m := loadTestManifest(t, root)
	if got, ok := manifestSetting(m, requireCinchKey); !ok || got != "1.2.4" {
		t.Fatalf("require.cinch = %q, ok=%v, want 1.2.4", got, ok)
	}
	if _, err := os.Stat(filepath.Join(root, defaultHooksPath, "pre-commit")); err != nil {
		t.Fatalf("expected hook shim to be rendered: %v", err)
	}
}

// cinch:rule CINCH-002
func TestCmdRenderRemovesOrphans(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "")
	directory := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	staleBody := "# Retired workflow\n"
	writeTestFile(t, directory, "retired.md", header("docs/templates/retired.md", staleBody, styleMarkdown)+staleBody)
	writeTestFile(t, directory, "hand-written.md", "# Ours\n")

	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(directory, "retired.md")); !os.IsNotExist(err) {
		t.Fatalf("retired.md stat err = %v, want it removed", err)
	}
	if got := readTestFile(t, directory, "hand-written.md"); got != "# Ours\n" {
		t.Fatalf("hand-written.md = %q, want it untouched", got)
	}
}
