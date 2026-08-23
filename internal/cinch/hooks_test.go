package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func checkHooksTest(t *testing.T, root string) checkResult {
	t.Helper()
	m, mErr := loadManifestOptional(root)
	return checkHooks(root, m, mErr)
}

func TestCheckHooksNotGitRepo(t *testing.T) {
	root := t.TempDir()
	res := checkHooksTest(t, root)
	if res.noOp != "not a git repository" {
		t.Fatalf("noOp = %q, want %q", res.noOp, "not a git repository")
	}
}

func TestCheckHooksNoManifest(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")

	res := checkHooksTest(t, root)
	if !strings.Contains(res.noOp, "cinch.yml not found") {
		t.Fatalf("noOp = %q, want it to contain %q", res.noOp, "cinch.yml not found")
	}
}

func TestCheckHooksUnset(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")

	res := checkHooksTest(t, root)
	if !strings.Contains(res.noOp, "not set") {
		t.Fatalf("noOp = %q, want it to contain %q", res.noOp, "not set")
	}
}

func TestCheckHooksMismatch(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", "other-hooks")

	res := checkHooksTest(t, root)
	if !strings.Contains(res.noOp, "expected") {
		t.Fatalf("noOp = %q, want it to contain %q", res.noOp, "expected")
	}
}

func TestCheckHooksMatch(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", ".githooks")

	res := checkHooksTest(t, root)
	if res.noOp != "" || len(res.findings) != 0 {
		t.Fatalf("checkHooks = %+v, want clean result", res)
	}
}

func TestCheckHooksDefaultPath(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	writeTestFile(t, root, "cinch.yml", "commit:\n  pattern: '^docs: .+'\n")
	git("config", "core.hooksPath", filepath.Clean(defaultHooksPath))

	res := checkHooksTest(t, root)
	if res.noOp != "" || len(res.findings) != 0 {
		t.Fatalf("checkHooks = %+v, want clean result using default paths.hooks", res)
	}
}

func gitTestHelper(t *testing.T, root string) func(args ...string) {
	t.Helper()
	return func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append([]string{
			"HOME=" + root,
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
		}, os.Environ()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
