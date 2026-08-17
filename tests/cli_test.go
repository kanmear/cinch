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

// TestShellUsage exercises bare `cinch` run with the test process's own
// cwd (tests/) — inside the cinch repo's git tree, but tests/ itself has no
// local cinch.yml, and CombinedOutput leaves Stdin nil (non-interactive).
// That's the "uninitialized, non-interactive" case: no prompt, just the
// informational line and the init/help hint, still exit 2.
func TestShellUsage(t *testing.T) {
	out, err := exec.Command("../bin/cinch").CombinedOutput()
	if err == nil {
		t.Fatalf("no-args run: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("no-args run: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "cinch init") || !strings.Contains(string(out), "cinch help") {
		t.Fatalf("no-args run: init/help hint not printed:\n%s", out)
	}
}

func TestHelp_PrintsCommandListAndExitsZero(t *testing.T) {
	out, err := exec.Command(binPath(t), "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help: want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "usage:") {
		t.Fatalf("help: command list not printed:\n%s", out)
	}
}

func TestHelpAliases(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			out, err := exec.Command(binPath(t), flag).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: want exit 0, got %v\n%s", flag, err, out)
			}
			if !strings.Contains(string(out), "usage:") {
				t.Fatalf("%s: command list not printed:\n%s", flag, out)
			}
		})
	}
}

// TestBareInvocation_Initialized covers the case a cinch.yml already exists:
// bare `cinch` should show the short synopsis pointing at `cinch help`, not
// the full command list.
func TestBareInvocation_Initialized(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write cinch.yml: %v", err)
	}

	cmd := exec.Command(binPath(t))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bare invocation, initialized: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("bare invocation, initialized: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "cinch help") {
		t.Fatalf("bare invocation, initialized: synopsis not printed:\n%s", out)
	}
	if strings.Contains(string(out), "cinch version") {
		t.Fatalf("bare invocation, initialized: want short synopsis, got full command list:\n%s", out)
	}
}

// TestBareInvocation_UninitializedNonGit covers a plain, non-git tempdir
// with no cinch.yml: the init prompt must never appear (not a git repo),
// and the run must complete without hanging on stdin.
func TestBareInvocation_UninitializedNonGit(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath(t))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bare invocation, uninitialized non-git: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("bare invocation, uninitialized non-git: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "doesn't look like an initialized cinch project") {
		t.Fatalf("bare invocation, uninitialized non-git: info line not printed:\n%s", out)
	}
	if !strings.Contains(string(out), "cinch init") || !strings.Contains(string(out), "cinch help") {
		t.Fatalf("bare invocation, uninitialized non-git: hint not printed:\n%s", out)
	}
}

// TestBareInvocation_UninitializedGitNonInteractive proves the init prompt
// requires BOTH a git repo and an interactive stdin — a git repo alone
// (with CombinedOutput's non-TTY stdin) must not trigger the prompt.
func TestBareInvocation_UninitializedGitNonInteractive(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bare invocation, uninitialized git non-interactive: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("bare invocation, uninitialized git non-interactive: want exit code 2, got %v", err)
	}
	if strings.Contains(string(out), "[y/N]") {
		t.Fatalf("bare invocation, uninitialized git non-interactive: prompt shown despite non-TTY stdin:\n%s", out)
	}
	if !strings.Contains(string(out), "cinch init") || !strings.Contains(string(out), "cinch help") {
		t.Fatalf("bare invocation, uninitialized git non-interactive: hint not printed:\n%s", out)
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

func TestVersion_PrintsAndExitsZero(t *testing.T) {
	out, err := exec.Command(binPath(t), "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: want exit 0, got %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 1 || strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("version: want exactly one non-empty line, got:\n%s", out)
	}
}

func TestVersion_TooManyArgsIsUsageError(t *testing.T) {
	out, err := exec.Command(binPath(t), "version", "extra").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("version with an argument: want exit 2, got %v\n%s", err, out)
	}
}

// TestVersion_DefaultsToDevel builds a binary with no -ldflags (bypassing
// the Makefile's VERSION injection) to confirm the fallback default — the
// pre-built ../bin/cinch (via `make test`) always carries a stamped version,
// so it can't exercise this path.
func TestVersion_DefaultsToDevel(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "cinch-devel")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build without ldflags: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: want exit 0, got %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "devel" {
		t.Fatalf("version with no ldflags: want %q, got %q", "devel", got)
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
	manifest := "paths:\n  docs: mydocs\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
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

// Replaces TestCheck_AbsoluteDocsRootFromManifest, which asserted that check
// scanned an absolute docs root — the read half of a split that let render
// write elsewhere and every check pass against a corpus none of them saw.
// The refusal has to be visible end to end through the binary, since that is
// the surface a consumer misconfigures.
func TestCheck_NonLocalDocsRootFromManifestFires(t *testing.T) {
	for _, val := range []string{filepath.Join(t.TempDir(), "elsewhere-docs"), "../elsewhere-docs"} {
		dir := t.TempDir()
		manifest := "paths:\n  docs: " + val + "\n"
		if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		cmd := exec.Command(binPath(t), "check")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 1 {
			t.Fatalf("check with paths.docs = %q: want exit 1, got %v\n%s", val, err, out)
		}
		if !strings.Contains(string(out), "paths.docs") || !strings.Contains(string(out), "outside the repository") {
			t.Fatalf("check with paths.docs = %q: refusal not named:\n%s", val, out)
		}
	}
}

func TestCheck_RequireCinchMatchIsClean(t *testing.T) {
	dir := t.TempDir()
	version := versionOf(t)
	manifest := "require:\n  cinch: " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check with matching require.cinch: want exit 0, got %v\n%s", err, out)
	}
}

func TestCheck_RequireCinchMismatchFires(t *testing.T) {
	dir := t.TempDir()
	manifest := "require:\n  cinch: 9.9.9\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("check with mismatched require.cinch: want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "core error") || !strings.Contains(string(out), "9.9.9") {
		t.Fatalf("check with mismatched require.cinch: finding not named:\n%s", out)
	}
}

func TestCheck_RequireCinchAbsentIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check with no require.cinch key: want exit 0, got %v\n%s", err, out)
	}
	if strings.Contains(string(out), "core error") {
		t.Fatalf("check with no require.cinch key: unexpected core finding:\n%s", out)
	}
}

// versionOf runs `cinch version` against the built binary, so the match
// fixture stays correct regardless of the Makefile's VERSION setting.
func versionOf(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(binPath(t), "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
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
	manifest := "paths:\n  docs: mydocs\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
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
		"mydocs/workflows/docs-philosophy.md",
		"mydocs/workflows/docs-maintain-domain.md",
		"mydocs/workflows/docs-audit-coverage.md",
		"mydocs/workflows/dev-execute-plan.md",
		"mydocs/workflows/dev-fix-bug.md",
		"mydocs/workflows/docs-audit-quality.md",
		"mydocs/workflows/dev-plan-feature.md",
		"mydocs/workflows/docs-sync.md",
		"mydocs/workflows/dev-task-primitive.md",
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

	philosophy := read("mydocs/workflows/docs-philosophy.md")
	if !strings.HasPrefix(philosophy, "<!-- generated by cinch") {
		t.Fatalf("rendered docs-philosophy.md missing generated header:\n%s", philosophy)
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
	if !strings.Contains(string(out), "cinch.yml") {
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

	manifest := "paths:\n" +
		"  docs: .docs\n" +
		"hooks:\n" +
		"  pre-commit:\n" +
		"    demo:\n" +
		"      run: \"false\"\n" +
		"      when: [src/]\n"
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte(manifest), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
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
	if !strings.Contains(string(out), "| Workflow | Trigger |") || !strings.Contains(string(out), "docs-maintain-domain.md") {
		t.Fatalf("workflows: missing expected content:\n%s", out)
	}
}

func TestWorkflows_RenderNotRunFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
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

func TestIndex_PrintsDocList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "index")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("index: want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "workflows/docs-maintain-domain.md") {
		t.Fatalf("index: missing expected content:\n%s", out)
	}
}

func TestContext_PrintsReport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	planPath := filepath.Join(dir, ".docs", "plans", "thing.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	if err := os.WriteFile(planPath, []byte("# A Thing\n\nStatus: **proposed**.\n"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	cmd := exec.Command(binPath(t), "context")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("context: want exit 0, got %v\n%s", err, out)
	}
	for _, want := range []string{"1 plan under", "A Thing", "Status: **proposed**.", "| Workflow | Trigger |"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("context: missing %q:\n%s", want, out)
		}
	}
}

// Unlike index and workflows, context does not fail when render hasn't run —
// it is the first thing a session runs, so a missing input is a stated skip
// and the remaining sections still print.
func TestContext_RenderNotRunStillExitsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "context")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("context with no render: want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "skip:") {
		t.Fatalf("context with no render: want a stated skip, got:\n%s", out)
	}
}

func TestIndex_RenderNotRunFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command(binPath(t), "index")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("index with no render: want exit 1, got %v\n%s", err, out)
	}
}

// TestWorkflow_CustomDocsRootFromManifest confirms `cinch workflow NAME`
// resolves with no path argument anywhere, even when paths.docs is
// customized — the whole point of routing consumer AGENTS.md files through
// a command instead of a hardcoded path.
func TestWorkflow_CustomDocsRootFromManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: mydocs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath(t), "workflow", "docs-maintain-domain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("workflow docs-maintain-domain: want exit 0, got %v\n%s", err, out)
	}
	want, err := os.ReadFile(filepath.Join(dir, "mydocs", "workflows", "docs-maintain-domain.md"))
	if err != nil {
		t.Fatalf("read rendered docs-maintain-domain.md: %v", err)
	}
	if string(out) != string(want) {
		t.Fatalf("workflow docs-maintain-domain: output doesn't match the rendered file on disk")
	}
}

func TestWorkflow_UnknownNameFires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
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

func TestMoveDocs_MissingArgIsUsageError(t *testing.T) {
	out, err := exec.Command(binPath(t), "move-docs").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("move-docs with no NEW-PATH: want exit 2, got %v\n%s", err, out)
	}
}

func TestMoveDocs_TooManyArgsIsUsageError(t *testing.T) {
	out, err := exec.Command(binPath(t), "move-docs", "a", "b").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("move-docs with two args: want exit 2, got %v\n%s", err, out)
	}
}

// TestMoveDocs_EndToEndGitRepo is the CLI-level rehearsal from the migration
// plan's verification section (B.1-3): `cinch move-docs` in a real git repo
// must move the docs root as a staged rename, rewrite cinch.yml, re-render,
// and leave `cinch check` clean.
func TestMoveDocs_EndToEndGitRepo(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "add", "-A"); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "commit", "-q", "-m", "seed"); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	moveCmd := exec.Command(binPath(t), "move-docs", ".docs2")
	moveCmd.Dir = dir
	if out, err := moveCmd.CombinedOutput(); err != nil {
		t.Fatalf("move-docs: want exit 0, got %v\n%s", err, out)
	}

	statusOut, err := runGit(t, dir, "diff", "--cached", "--name-status")
	if err != nil {
		t.Fatalf("git diff --cached: %v\n%s", err, statusOut)
	}
	if !strings.Contains(string(statusOut), "R") {
		t.Fatalf("want a staged rename after move-docs, got:\n%s", statusOut)
	}

	manifest, err := os.ReadFile(filepath.Join(dir, "cinch.yml"))
	if err != nil {
		t.Fatalf("read cinch.yml: %v", err)
	}
	if !strings.Contains(string(manifest), "docs: .docs2") {
		t.Fatalf("cinch.yml: want paths.docs rewritten to .docs2, got:\n%s", manifest)
	}
	if _, err := os.Stat(filepath.Join(dir, ".docs")); err == nil {
		t.Fatalf("old .docs directory should be gone")
	}

	checkCmd := exec.Command(binPath(t), "check")
	checkCmd.Dir = dir
	var stdout bytes.Buffer
	checkCmd.Stdout = &stdout
	if err := checkCmd.Run(); err != nil {
		t.Fatalf("check after move-docs: want exit 0, got %v\nfindings:\n%s", err, stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("check after move-docs: want zero findings, got:\n%s", stdout.String())
	}
}

// TestMoveDocs_UncommittedRenameWithoutMoveIsCaught is the negative-path
// rehearsal from the migration plan's verification section (B.4): the exact
// mistake `cinch move-docs` exists to prevent — editing paths.docs by hand
// without moving the directory — must now be caught by `cinch check`.
func TestMoveDocs_UncommittedRenameWithoutMoveIsCaught(t *testing.T) {
	dir := t.TempDir()
	if out, err := runGit(t, dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	renderCmd := exec.Command(binPath(t), "render")
	renderCmd.Dir = dir
	if out, err := renderCmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "add", "-A"); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if out, err := runGit(t, dir, "commit", "-q", "-m", "seed"); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	// Hand-edit paths.docs without moving .docs/workflows/*.md — the mistake,
	// not the fix.
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: .docs2\n"), 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	checkCmd := exec.Command(binPath(t), "check")
	checkCmd.Dir = dir
	out, _ := checkCmd.CombinedOutput()
	if !strings.Contains(string(out), "orphaned") {
		t.Fatalf("check after an uncommitted paths.docs rename without moving the directory: want orphan findings, got:\n%s", out)
	}
}
