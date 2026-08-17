package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdMoveDocs_GitRepoMovesAndRerenders(t *testing.T) {
	dir := gitInitRepo(t)
	renderToScratch(t, dir)
	gitCommitAll(t, dir, "seed")

	if code := CmdMoveDocs(dir, ".docs2"); code != 0 {
		t.Fatalf("CmdMoveDocs: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, ".docs")); err == nil {
		t.Fatalf("old .docs directory should be gone after move")
	}
	if _, err := os.Stat(filepath.Join(dir, ".docs2", "workflows")); err != nil {
		t.Fatalf(".docs2/workflows should exist after move: %v", err)
	}

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars[pathsDocsKey] != ".docs2" {
		t.Fatalf("paths.docs: want %q, got %q", ".docs2", m.Vars[pathsDocsKey])
	}

	cmd := exec.Command("git", "diff", "--cached", "--name-status")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff --cached: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "R") {
		t.Fatalf("want a staged rename in `git diff --cached --name-status`, got:\n%s", out)
	}
}

func TestCmdMoveDocs_NonGitRepoFallsBackToRename(t *testing.T) {
	dir := t.TempDir() // no git init
	renderToScratch(t, dir)

	if code := CmdMoveDocs(dir, ".docs2"); code != 0 {
		t.Fatalf("CmdMoveDocs: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, ".docs")); err == nil {
		t.Fatalf("old .docs directory should be gone after move")
	}
	if _, err := os.Stat(filepath.Join(dir, ".docs2", "workflows")); err != nil {
		t.Fatalf(".docs2/workflows should exist after move: %v", err)
	}

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars[pathsDocsKey] != ".docs2" {
		t.Fatalf("paths.docs: want %q, got %q", ".docs2", m.Vars[pathsDocsKey])
	}
}

func TestCmdMoveDocs_RefusesToOverwriteExistingTarget(t *testing.T) {
	dir := t.TempDir()
	renderToScratch(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".docs2"), 0o755); err != nil {
		t.Fatalf("mkdir .docs2: %v", err)
	}

	if code := CmdMoveDocs(dir, ".docs2"); code == 0 {
		t.Fatalf("CmdMoveDocs onto an existing target: want non-zero exit, got 0")
	}

	if _, err := os.Stat(filepath.Join(dir, ".docs")); err != nil {
		t.Fatalf(".docs should be untouched after a refused move: %v", err)
	}
	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars[pathsDocsKey] != ".docs" {
		t.Fatalf("paths.docs should be untouched after a refused move, got %q", m.Vars[pathsDocsKey])
	}
}

func TestCmdMoveDocs_NoOpWhenAlreadyAtTarget(t *testing.T) {
	dir := t.TempDir()
	renderToScratch(t, dir)

	if code := CmdMoveDocs(dir, ".docs"); code != 0 {
		t.Fatalf("CmdMoveDocs to the current value: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, ".docs", "workflows")); err != nil {
		t.Fatalf(".docs/workflows should still exist: %v", err)
	}
	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars[pathsDocsKey] != ".docs" {
		t.Fatalf("paths.docs: want unchanged %q, got %q", ".docs", m.Vars[pathsDocsKey])
	}
}
