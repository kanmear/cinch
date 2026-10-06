package cinch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cinch/internal/gitutil"
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

// cinch:rule CINCH-008
func TestCmdInitWritesFloorPin(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    string
		pinned  bool
	}{
		{version: "1.2.3", want: ">=1.2.3", pinned: true},
		{version: "dev", pinned: false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			withVersion(t, tc.version)
			root := t.TempDir()
			git := gitTestHelper(t, root)
			git("init", "-q")

			if code := CmdInit(root); code != 0 {
				t.Fatalf("CmdInit = %d, want 0", code)
			}

			got, ok := manifestSetting(loadTestManifest(t, root), requireCinchKey)
			if ok != tc.pinned || got != tc.want {
				t.Fatalf("require.cinch = %q, ok=%v, want %q, ok=%v", got, ok, tc.want, tc.pinned)
			}
		})
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

// cinch:rule CINCH-003
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

func TestCmdInitNormalizesAbsoluteEquivalentHooksPath(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", filepath.Join(root, ".githooks"))

	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit = %d, want 0 when core.hooksPath is the absolute spelling of paths.hooks", code)
	}

	got := gitConfigGet(t, root, "core.hooksPath")
	if got != ".githooks" {
		t.Fatalf("core.hooksPath = %q, want normalized to %q", got, ".githooks")
	}
}

func gitConfigGet(t *testing.T, root, key string) string {
	t.Helper()
	out, err := gitutil.Output(root, "config", "--get", key)
	if err != nil {
		t.Fatalf("git config --get %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func initAgentsRepo(t *testing.T, manifestBody string) string {
	t.Helper()
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	writeTestFile(t, root, "cinch.yml", manifestBody)
	return root
}

func readAgentsFile(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustCmdInit(t *testing.T, root string) {
	t.Helper()
	if code := CmdInit(root); code != 0 {
		t.Fatalf("CmdInit = %d, want 0", code)
	}
}

func TestCmdInitWritesAgentsPointerWhenMissing(t *testing.T) {
	root := initAgentsRepo(t, "paths:\n  docs: docs/ops\n")

	mustCmdInit(t, root)

	got := string(readAgentsFile(t, root))
	if !strings.HasPrefix(got, "# Agent instructions\n\n## Project docs (cinch)\n") {
		t.Fatalf("AGENTS.md = %q, want heading then cinch block", got)
	}
	if !strings.Contains(got, "live in `docs/ops/`") {
		t.Fatalf("AGENTS.md = %q, want the custom docs path", got)
	}
}

func TestCmdInitAppendsAgentsPointer(t *testing.T) {
	cases := []struct {
		name     string
		original string
	}{
		{"trailing newline", "# Mine\n\nBe nice.\n"},
		{"no trailing newline", "# Mine\n\nBe nice."},
		{"trailing blank line", "# Mine\n\nBe nice.\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initAgentsRepo(t, "paths:\n  docs: .docs\n")
			writeTestFile(t, root, "AGENTS.md", tc.original)

			mustCmdInit(t, root)

			got := string(readAgentsFile(t, root))
			if !strings.HasPrefix(got, tc.original) {
				t.Fatalf("AGENTS.md = %q, want original %q as prefix", got, tc.original)
			}
			if !strings.Contains(got, "\n\n## Project docs (cinch)\n") {
				t.Fatalf("AGENTS.md = %q, want block after one blank line", got)
			}
			if strings.Contains(got, "\n\n\n") {
				t.Fatalf("AGENTS.md = %q, want no doubled blank lines", got)
			}
		})
	}
}

func TestCmdInitLeavesAgentsFileMentioningCinch(t *testing.T) {
	root := initAgentsRepo(t, "paths:\n  docs: .docs\n")
	original := "# Mine\n\nRun Cinch check before committing.\n"
	writeTestFile(t, root, "AGENTS.md", original)

	mustCmdInit(t, root)

	if got := string(readAgentsFile(t, root)); got != original {
		t.Fatalf("AGENTS.md = %q, want unchanged %q", got, original)
	}
}

func TestCmdInitAgentsPointerIdempotent(t *testing.T) {
	root := initAgentsRepo(t, "paths:\n  docs: .docs\n")

	mustCmdInit(t, root)
	first := readAgentsFile(t, root)
	mustCmdInit(t, root)

	if second := readAgentsFile(t, root); !bytes.Equal(first, second) {
		t.Fatalf("AGENTS.md changed on second init:\n%s\n---\n%s", first, second)
	}
}

func TestCmdInitAgentsPointerOptOut(t *testing.T) {
	root := initAgentsRepo(t, "paths:\n  docs: .docs\nagents:\n  pointer: false\n")

	stderr := captureStderr(t, func() { mustCmdInit(t, root) })

	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md stat err = %v, want not-exist", err)
	}
	if !strings.Contains(stderr, "agents.pointer is false") {
		t.Fatalf("stderr = %q, want a skip line", stderr)
	}
}

func TestAgentPointerBlockHasNoPhantomMarkers(t *testing.T) {
	block := agentsHeading + strings.Replace(agentPointerTemplate, "%s", ".docs", 1)

	markers, err := scanMarkers(strings.NewReader(block), "AGENTS.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
		t.Fatalf("scanMarkers = %v, want none", markers)
	}
	if items := parseRuleItems("AGENTS.md", []byte(block)); len(items) != 0 {
		t.Fatalf("parseRuleItems = %v, want none", items)
	}
}

func TestCmdInitAgentsPointerKeepsRulesCheckSkipped(t *testing.T) {
	root := initAgentsRepo(t, "paths:\n  docs: .docs\n")
	mustCmdInit(t, root)

	var code int
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code = runChecks(checkRoots{repoRoot: root, fsRoot: root}, "", false, "rules")
		})
	})

	if code != 0 {
		t.Fatalf("runChecks = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want no findings", stdout)
	}
	if !strings.Contains(stderr, "rules: ") || !strings.Contains(stderr, "skip: ") {
		t.Fatalf("stderr = %q, want a rules skip", stderr)
	}
}

func TestCmdInitClaudeMdHint(t *testing.T) {
	const hint = "CLAUDE.md doesn't reference AGENTS.md"
	cases := []struct {
		name     string
		claude   string
		wantHint bool
	}{
		{"lacks reference", "# Claude\n\nBe brief.\n", true},
		{"references AGENTS.md", "@AGENTS.md\n", false},
		{"mentions cinch", "Use cinch.\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initAgentsRepo(t, "paths:\n  docs: .docs\n")
			writeTestFile(t, root, "CLAUDE.md", tc.claude)

			stderr := captureStderr(t, func() { mustCmdInit(t, root) })

			if got := strings.Contains(stderr, hint); got != tc.wantHint {
				t.Fatalf("hint printed = %v, want %v\nstderr: %s", got, tc.wantHint, stderr)
			}
			data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil || string(data) != tc.claude {
				t.Fatalf("CLAUDE.md = %q (err %v), want unchanged %q", data, err, tc.claude)
			}
		})
	}
}
