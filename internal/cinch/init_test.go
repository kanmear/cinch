package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdInit_WritesStarterManifestWhenAbsent(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(root, "cinch.yml")); err != nil {
		t.Fatalf("want cinch.yml written: %v", err)
	}
}

func TestCmdInit_KeepsExistingManifest(t *testing.T) {
	root := gitInitRepo(t)
	custom := "paths:\n  docs: mydocs\n"
	writeFile(t, filepath.Join(root, "cinch.yml"), custom)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	got, err := os.ReadFile(filepath.Join(root, "cinch.yml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(got) != custom {
		t.Fatalf("want existing manifest untouched, got:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "mydocs", "plans", ".gitkeep")); err != nil {
		t.Fatalf("want plans dir created under the custom paths.docs: %v", err)
	}
}

func TestCmdInit_WritesManifestExampleWhenAbsent(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	data, err := os.ReadFile(filepath.Join(root, "manifest.example.yml"))
	if err != nil {
		t.Fatalf("read manifest.example.yml: %v", err)
	}
	if !strings.Contains(string(data), "cinch never reads this file") {
		t.Fatalf("want manifest.example.yml to carry its explanatory header, got:\n%s", data)
	}
}

func TestCmdInit_LeavesExistingManifestExampleUntouched(t *testing.T) {
	root := gitInitRepo(t)
	custom := "# my own manifest\nproject:\n  name: custom\n"
	writeFile(t, filepath.Join(root, "manifest.example.yml"), custom)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	got, err := os.ReadFile(filepath.Join(root, "manifest.example.yml"))
	if err != nil {
		t.Fatalf("read manifest.example.yml: %v", err)
	}
	if string(got) != custom {
		t.Fatalf("want existing manifest.example.yml untouched, got:\n%s", got)
	}
}

func TestCmdInit_CreatesPlansDirWithGitkeep(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(root, ".docs", "plans", ".gitkeep")); err != nil {
		t.Fatalf("want .docs/plans/.gitkeep created: %v", err)
	}
}

func TestCmdInit_ActivatesHooksInGitRepo(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	out, err := gitOutputLines(root, "config", "--get", "core.hooksPath")
	if err != nil {
		t.Fatalf("git config --get core.hooksPath: %v", err)
	}
	if len(out) != 1 || out[0] != ".githooks" {
		t.Fatalf("want core.hooksPath = .githooks, got %v", out)
	}
}

func TestCmdInit_NotAGitRepoStillRendersShims(t *testing.T) {
	root := t.TempDir()

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(root, ".githooks", "pre-commit")); err != nil {
		t.Fatalf("want hook shim generated even outside a git repo: %v", err)
	}
}

func TestCmdInit_WritesAgentsMDWhenAbsent(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	// All three command pointers, not just the workflows one: a consumer
	// whose AGENTS.md omits `cinch context` has no session-start entry point,
	// which is the one line an agent reads first.
	for _, line := range []string{agentsContextLine, agentsWorkflowsLine, agentsIndexLine} {
		if !strings.Contains(string(data), line) {
			t.Fatalf("want AGENTS.md to carry %q, got:\n%s", line, data)
		}
	}
}

func TestCmdInit_LeavesExistingAgentsMDUntouched(t *testing.T) {
	root := gitInitRepo(t)
	custom := "# My Project\n\nCustom content.\n"
	writeFile(t, filepath.Join(root, "AGENTS.md"), custom)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit: want exit 0, got %d", code)
	}

	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != custom {
		t.Fatalf("want existing AGENTS.md untouched, got:\n%s", got)
	}
}

func TestCmdInit_TwiceIsByteIdentical(t *testing.T) {
	root := gitInitRepo(t)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit (first): want exit 0, got %d", code)
	}
	before := snapshotTree(t, root)

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit (second): want exit 0, got %d", code)
	}
	after := snapshotTree(t, root)

	if len(before) != len(after) {
		t.Fatalf("init twice: file count changed: %d vs %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("init twice: %s not byte-identical", path)
		}
	}
}

// snapshotTree reads every regular file under root (excluding .git) into a
// map keyed by its path relative to root.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotTree: %v", err)
	}
	return out
}
