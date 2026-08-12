// Package tests exercises the built cinch binary end-to-end (`make test` builds
// bin/cinch before testing).
package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellUsage(t *testing.T) {
	out, err := exec.Command("../bin/cinch").CombinedOutput()
	if err == nil {
		t.Fatalf("no-args run: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("no-args run: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "usage:") {
		t.Fatalf("no-args run: usage not printed:\n%s", out)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	out, err := exec.Command("../bin/cinch", "bogus").CombinedOutput()
	if err == nil {
		t.Fatalf("unknown command: want non-zero exit, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("unknown command: want exit code 1, got %v", err)
	}
	if !strings.Contains(string(out), `"bogus" is not a command`) {
		t.Fatalf("unknown command: notice not printed:\n%s", out)
	}
}

func TestCheckNoDocsDir(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		t.Fatalf("check with no .docs dir: want exit 0, got %v", err)
	}
	// No findings on stdout — a plain, non-git tempdir with no .docs is a
	// clean pass. Coupling still writes its no-op notice to stderr (not
	// asserted here); silence there would misread as "checked and clean".
	if stdout.Len() != 0 {
		t.Fatalf("check with no .docs dir: want no findings on stdout, got:\n%s", stdout.String())
	}
}

func TestCheckExitCodesEndToEnd(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir .docs: %v", err)
	}
	content := "see [x](./missing.md) for details\n"
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check with broken link: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "links error") || !strings.Contains(string(out), "missing.md") {
		t.Fatalf("check with broken link: finding not named:\n%s", out)
	}
}

func TestCheck_CustomDocsRootFromManifest(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "mydocs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir mydocs: %v", err)
	}
	content := "see [x](./missing.md) for details\n"
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}
	manifest := "paths.docs = mydocs\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check against custom docs root: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "links error") || !strings.Contains(string(out), "mydocs") {
		t.Fatalf("check against custom docs root: finding not under mydocs:\n%s", out)
	}

	// A default .docs dir sitting alongside the configured one must be
	// ignored entirely — the manifest key wins outright, not additively.
	unusedDocs := filepath.Join(dir, ".docs")
	if err := os.MkdirAll(unusedDocs, 0o755); err != nil {
		t.Fatalf("mkdir .docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unusedDocs, "b.md"), []byte("see [y](./gone.md)\n"), 0o644); err != nil {
		t.Fatalf("write b.md: %v", err)
	}
	cmd = exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	ee, ok = err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check with unused .docs present: want exit 1, got %v\n%s", err, out)
	}
	if strings.Contains(string(out), "gone.md") {
		t.Fatalf("check with unused .docs present: unused .docs was scanned:\n%s", out)
	}
	if !strings.Contains(string(out), "missing.md") {
		t.Fatalf("check with unused .docs present: expected mydocs finding missing:\n%s", out)
	}
}

func TestCheck_AbsoluteDocsRootFromManifest(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(t.TempDir(), "elsewhere-docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir elsewhere-docs: %v", err)
	}
	content := "see [x](./missing.md) for details\n"
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}
	manifest := "paths.docs = " + docs + "\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check against absolute docs root: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "links error") || !strings.Contains(string(out), "missing.md") {
		t.Fatalf("check against absolute docs root: finding not named:\n%s", out)
	}
}

func TestIgnores_ListsDeclarationsWithReasons(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir .docs: %v", err)
	}
	content := "1. **CAT-003** tab display names derive from category name.\n" +
		"   <!-- cinch:ignore: UI-derivation fact, no test to point at -->\n" +
		"2. **MEMB-006** asserts the absence of a flow.\n" +
		"   <!-- cinch:ignore: asserts absence of a flow -->\n"
	if err := os.WriteFile(filepath.Join(docs, "rules.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write rules.md: %v", err)
	}

	cmd := exec.Command(binPath(t), "ignores")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ignores: want exit 0, got %v\n%s", err, out)
	}
	for _, want := range []string{"CAT-003", "UI-derivation fact", "MEMB-006", "absence of a flow"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("ignores: output missing %q:\n%s", want, out)
		}
	}
}

func TestCheckMsgFileArg(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(binPath(t), "check", "/nonexistent/msg/file")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("check with unreadable MSGFILE: want exit 2, got %v\n%s", err, out)
	}

	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("a commit message\n"), 0o644); err != nil {
		t.Fatalf("write msg file: %v", err)
	}
	cmd = exec.Command(binPath(t), "check", msgFile)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("check with a readable MSGFILE: want exit 0, got %v", err)
	}

	cmd = exec.Command(binPath(t), "check", msgFile, "extra")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	ee, ok = err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("check with too many args: want exit 2, got %v\n%s", err, out)
	}
}

func TestRender_EndToEndAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	manifest := "paths.docs = mydocs\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	run := func() []byte {
		cmd := exec.Command(binPath(t), "render")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("render: want exit 0, got %v\n%s", err, out)
		}
		return out
	}

	wantNamed := []string{
		"mydocs/workflows/doc-philosophy.md",
		"mydocs/workflows/maintain-domain.md",
		"mydocs/workflows/audit-domain.md",
		"mydocs/workflows/execute-plan.md",
		"mydocs/workflows/fix-bug.md",
		"mydocs/workflows/audit-docs.md",
		"mydocs/workflows/plan-feature.md",
		"mydocs/workflows/sync-docs.md",
		"mydocs/workflows/task-primitive.md",
	}

	out := run()
	for _, dest := range wantNamed {
		if !strings.Contains(string(out), dest) {
			t.Fatalf("render: expected output did not name %s:\n%s", dest, out)
		}
	}

	read := func(dest string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, dest))
		if err != nil {
			t.Fatalf("read rendered %s: %v", dest, err)
		}
		return string(b)
	}

	philosophy := read("mydocs/workflows/doc-philosophy.md")
	if !strings.HasPrefix(philosophy, "<!-- generated by cinch") {
		t.Fatalf("rendered doc-philosophy.md missing generated header:\n%s", philosophy)
	}
	before := map[string]string{}
	for _, dest := range wantNamed {
		body := read(dest)
		if strings.Contains(body, "{{") {
			t.Fatalf("rendered %s has an unresolved {{}} token", dest)
		}
		before[dest] = body
	}

	run() // re-render
	for _, dest := range wantNamed {
		if before[dest] != read(dest) {
			t.Fatalf("render: %s not idempotent across two runs", dest)
		}
	}
}

func TestRender_MissingManifestFires(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath(t), "render")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("render with no manifest: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "cinch_manifest") {
		t.Fatalf("render with no manifest: error doesn't name the file:\n%s", out)
	}
}

// binPath returns the absolute path to the built cinch binary. Tests that
// set cmd.Dir need this rather than a relative path: exec.Command resolves a
// relative Path against cmd.Dir, not the test process's own working
// directory.
func binPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../bin/cinch")
	if err != nil {
		t.Fatalf("resolve bin path: %v", err)
	}
	return abs
}

// gitEnv runs git non-interactively against a throwaway identity, with
// bin/'s directory prepended to PATH so a generated hook shim's `exec cinch
// hook ...` resolves to the binary this test suite just built.
func gitEnv(t *testing.T) []string {
	t.Helper()
	return append(os.Environ(),
		"PATH="+filepath.Dir(binPath(t))+":"+os.Getenv("PATH"),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=cinch-test",
		"GIT_AUTHOR_EMAIL=cinch-test@example.com",
		"GIT_COMMITTER_NAME=cinch-test",
		"GIT_COMMITTER_EMAIL=cinch-test@example.com",
	)
}

func runGit(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(t)
	return cmd.CombinedOutput()
}

// TestHook_PreCommitDispatchesByWhen is the load-bearing proof that the
// generated shim, core.hooksPath, and cinch hook's `when` prefix matching
// all actually wire together against a real `git commit` — not just the
// dispatch logic in isolation (hook_test.go covers that).
func TestHook_PreCommitDispatchesByWhen(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	manifest := "paths.docs = .docs\n" +
		"hooks.pre-commit.demo.run  = false\n" +
		"hooks.pre-commit.demo.when = src/\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	if out, err := runGit(t, dir, "config", "core.hooksPath", ".githooks"); err != nil {
		t.Fatalf("git config core.hooksPath: %v\n%s", err, out)
	}

	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write src/app.go: %v", err)
	}
	if out, err := runGit(t, dir, "add", "src/app.go"); err != nil {
		t.Fatalf("git add src/app.go: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "commit", "-q", "-m", "add src"); err == nil {
		t.Fatalf("commit under src/: want blocked by the registered script, got success\n%s", out)
	}
	// A blocked commit does not clear the index — unstage src/app.go so the
	// next commit's staged set doesn't still include it.
	if out, err := runGit(t, dir, "rm", "--cached", "-q", "src/app.go"); err != nil {
		t.Fatalf("git rm --cached src/app.go: %v\n%s", err, out)
	}

	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "notes.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("write docs/notes.md: %v", err)
	}
	if out, err := runGit(t, dir, "add", "docs/notes.md"); err != nil {
		t.Fatalf("git add docs/notes.md: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "commit", "-q", "-m", "add docs"); err != nil {
		t.Fatalf("commit under docs/ (script's when untouched): want success, got blocked: %v\n%s", err, out)
	}
}

// TestInit_ThenCheckExitsClean is the load-bearing proof the whole plan
// hinges on: `cinch init` in an empty repo must leave a tree `cinch check`
// passes with no configuration. If this holds, the release works.
func TestInit_ThenCheckExitsClean(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	initCmd := exec.Command(binPath(t), "init")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init: want exit 0, got %v\n%s", err, out)
	}

	checkCmd := exec.Command(binPath(t), "check")
	checkCmd.Dir = dir
	if out, err := checkCmd.CombinedOutput(); err != nil {
		t.Fatalf("check after init: want exit 0, got %v\n%s", err, out)
	}
}

// TestInit_TwiceIsByteIdentical is the CLI-level twin of
// TestCmdInit_TwiceIsByteIdentical (internal/cinch/init_test.go), driving
// the real binary rather than calling CmdInit directly.
func TestInit_TwiceIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	run := func() {
		cmd := exec.Command(binPath(t), "init")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("init: want exit 0, got %v\n%s", err, out)
		}
	}

	snapshot := func() map[string][]byte {
		out := map[string][]byte{}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = data
			return nil
		})
		return out
	}

	run()
	before := snapshot()
	run()
	after := snapshot()

	if len(before) != len(after) {
		t.Fatalf("init twice: file count changed: %d vs %d", len(before), len(after))
	}
	for path, content := range before {
		if string(after[path]) != string(content) {
			t.Fatalf("init twice: %s not byte-identical", path)
		}
	}
}

func TestWorkflows_PrintsTriggerTable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "workflows")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("workflows: want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "| Workflow | Trigger |") || !strings.Contains(string(out), "maintain-domain.md") {
		t.Fatalf("workflows: missing expected content:\n%s", out)
	}
}

func TestWorkflows_RenderNotRunFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "workflows")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("workflows with no render: want exit 1, got %v\n%s", err, out)
	}
}

func TestDocs_PrintsDocList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "docs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docs: want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "workflows/maintain-domain.md") {
		t.Fatalf("docs: missing expected content:\n%s", out)
	}
}

func TestDocs_RenderNotRunFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "docs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("docs with no render: want exit 1, got %v\n%s", err, out)
	}
}

// TestWorkflow_CustomDocsRootFromManifest confirms `cinch workflow NAME`
// resolves with no path argument anywhere, even when paths.docs is
// customized — the whole point of routing consumer AGENTS.md files through
// a command instead of a hardcoded path.
func TestWorkflow_CustomDocsRootFromManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = mydocs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "workflow", "maintain-domain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("workflow maintain-domain: want exit 0, got %v\n%s", err, out)
	}
	want, err := os.ReadFile(filepath.Join(dir, "mydocs", "workflows", "maintain-domain.md"))
	if err != nil {
		t.Fatalf("read rendered maintain-domain.md: %v", err)
	}
	if string(out) != string(want) {
		t.Fatalf("workflow maintain-domain: output doesn't match the rendered file on disk")
	}
}

func TestWorkflow_UnknownNameFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch_manifest"), []byte("paths.docs = .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "workflow", "bogus")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("workflow bogus: want exit 1, got %v\n%s", err, out)
	}
}

func TestWorkflow_MissingNameArgIsUsageError(t *testing.T) {
	out, err := exec.Command(binPath(t), "workflow").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("workflow with no NAME: want exit 2, got %v\n%s", err, out)
	}
}

func TestWorkflow_TooManyArgsIsUsageError(t *testing.T) {
	out, err := exec.Command(binPath(t), "workflow", "a", "b").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("workflow with two args: want exit 2, got %v\n%s", err, out)
	}
}
