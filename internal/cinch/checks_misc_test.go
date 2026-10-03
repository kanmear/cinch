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

func writeTestFile(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadTestManifest(t *testing.T, directory string) *manifest {
	t.Helper()
	m, err := loadManifestOptional(directory)
	if err != nil {
		t.Fatalf("loadManifestOptional(%s) = %v", directory, err)
	}
	return m
}

func TestCheckCommit(t *testing.T) {
	t.Run("no message file", func(t *testing.T) {
		directory := t.TempDir()
		r := checkCommit("", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("pattern not configured", func(t *testing.T) {
		directory := t.TempDir()
		r := checkCommit("msg.txt", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '['\n")
		r := checkCommit("msg.txt", loadTestManifest(t, directory))
		if len(r.findings) != 1 || r.findings[0].check != "commit" {
			t.Fatalf("result = %+v, want commit finding", r)
		}
	})

	t.Run("unreadable message file", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat'\n")
		r := checkCommit(filepath.Join(directory, "missing.txt"), loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("matches subject only", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, directory, "msg.txt", "feat: add things\n\nbody line\n")
		r := checkCommit(msg, loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("does not match", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, directory, "msg.txt", "fix: nope\n")
		r := checkCommit(msg, loadTestManifest(t, directory))
		if len(r.findings) != 1 {
			t.Fatalf("result = %+v, want one finding", r)
		}
		f := r.findings[0]
		if f.check != "commit" || f.level != "error" || f.file != msg || f.line != 1 {
			t.Fatalf("finding = %+v", f)
		}
	})
}

func TestCheckPin(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		directory := t.TempDir()
		r := checkPin("1.0.0", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("dev build skips the check", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.0.0\n")
		r := checkPin("dev", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("exact match", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.2.3\n")
		r := checkPin("1.2.3", loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("exact mismatch", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.2.3\n")
		r := checkPin("1.2.4", loadTestManifest(t, directory))
		if len(r.findings) != 1 || r.findings[0].check != "core" {
			t.Fatalf("result = %+v, want one core finding", r)
		}
	})

	t.Run("minimum satisfied", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
		r := checkPin("1.2.3", loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("minimum satisfied exactly", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=1.2.3'\n")
		r := checkPin("1.2.3", loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("minimum not satisfied", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=2.0.0'\n")
		r := checkPin("1.9.9", loadTestManifest(t, directory))
		if len(r.findings) != 1 || r.findings[0].check != "core" {
			t.Fatalf("result = %+v, want one core finding", r)
		}
		if want := "does not satisfy require.cinch >=2.0.0"; !strings.Contains(r.findings[0].message, want) {
			t.Fatalf("message = %q, want it to contain %q", r.findings[0].message, want)
		}
	})
}

// --- Seeded defect battery: generated.go / header.go ---
//
// Each fixture renders a real tree via CmdRender, then perturbs it the way
// a human edit or a stale checkout would, and asserts checkGenerated
// catches it.

func renderedFixture(t *testing.T) (root string, m *manifest, files []renderFile) {
	t.Helper()
	root = t.TempDir()
	writeTestFile(t, root, "cinch.yml", "")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	m = loadTestManifest(t, root)
	var err error
	files, err = renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("renderAll returned no files")
	}
	return root, m, files
}

func TestSeededDefectGeneratedHandEdited(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	path := filepath.Join(root, target.destination)

	original := readTestFile(t, root, target.destination)
	if err := os.WriteFile(path, []byte(original+"\nhand-edited line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(workingTreeRoots(root), m, nil)

	ok := false
	for _, f := range result.findings {
		if f.file == target.destination && strings.Contains(f.message, "does not match a fresh render") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'does not match a fresh render' finding for %s", result.findings, target.destination)
	}
}

func TestSeededDefectGeneratedMissingFile(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	path := filepath.Join(root, target.destination)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(workingTreeRoots(root), m, nil)

	ok := false
	for _, f := range result.findings {
		if f.file == target.destination && strings.Contains(f.message, "missing — run 'cinch render'") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'missing' finding for %s", result.findings, target.destination)
	}
}

func TestSeededDefectGeneratedOrphanFile(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	dir := filepath.Dir(filepath.Join(root, target.destination))

	orphanContent := header("orphan-source.md", "orphan body", styleMarkdown) + "orphan body"
	orphanPath := filepath.Join(dir, "orphan-leftover.md")
	if err := os.WriteFile(orphanPath, []byte(orphanContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(workingTreeRoots(root), m, nil)

	ok := false
	for _, f := range result.findings {
		if strings.Contains(f.file, "orphan-leftover.md") && strings.Contains(f.message, "orphaned generated file") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want an 'orphaned generated file' finding for orphan-leftover.md", result.findings)
	}
}
