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
		"mydocs/workflows/index.md",
		"mydocs/index.md",
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
	index := read("mydocs/workflows/index.md")
	if !strings.HasPrefix(index, "<!-- generated by cinch") {
		t.Fatalf("rendered index.md missing generated header:\n%s", index)
	}
	docMap := read("mydocs/index.md")
	if !strings.HasPrefix(docMap, "<!-- generated by cinch") {
		t.Fatalf("rendered doc map missing generated header:\n%s", docMap)
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
