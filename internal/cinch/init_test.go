package cinch

import (
	"strings"
	"testing"
)

func TestCmdInitActivatesHooksWhenUnset(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit = %d, want 0", code)
	}

	got := gitConfigGet(t, root, "core.hooksPath")
	if got != ".githooks" {
		t.Fatalf("core.hooksPath = %q, want %q", got, ".githooks")
	}
}

func TestCmdInitIdempotentWhenAlreadyMatching(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", ".githooks")

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit = %d, want 0 on a re-run matching paths.hooks", code)
	}

	got := gitConfigGet(t, root, "core.hooksPath")
	if got != ".githooks" {
		t.Fatalf("core.hooksPath = %q, want unchanged %q", got, ".githooks")
	}
}

func TestCmdInitRefusesToClobberExistingHooksPath(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", "other-hooks")

	if code := CmdInit(root); code == 0 {
		t.Fatalf("CmdInit = %d, want non-zero when core.hooksPath already points elsewhere", code)
	}

	got := gitConfigGet(t, root, "core.hooksPath")
	if got != "other-hooks" {
		t.Fatalf("core.hooksPath = %q, want left unchanged at %q", got, "other-hooks")
	}
}

func gitConfigGet(t *testing.T, root, key string) string {
	t.Helper()
	out, err := gitOutput(root, "config", "--get", key)
	if err != nil {
		t.Fatalf("git config --get %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}
