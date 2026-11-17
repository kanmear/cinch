package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func checkHooksTest(t *testing.T, repoRoot string) checkResult {
	t.Helper()
	m, mErr := loadManifestOptional(repoRoot)
	return checkHooks(repoRoot, m, mErr)
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

// catch-matrix: hooks | warns | core.hooksPath unset, so no hook runs
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

// catch-matrix: hooks | warns | core.hooksPath points somewhere other than paths.hooks
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

func TestCheckHooksMatchAbsolutePath(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	writeTestFile(t, root, "cinch.yml", "paths:\n  hooks: .githooks\n")
	git("config", "core.hooksPath", filepath.Join(root, ".githooks"))

	res := checkHooksTest(t, root)
	if res.noOp != "" || len(res.findings) != 0 {
		t.Fatalf("checkHooks = %+v, want clean result for the absolute spelling of paths.hooks", res)
	}
}

func TestSameHooksDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".githooks"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tests := []struct {
		name      string
		got, want string
		same      bool
	}{
		{"identical relative", ".githooks", ".githooks", true},
		{"unclean relative", "./.githooks/", ".githooks", true},
		{"absolute versus relative", filepath.Join(root, ".githooks"), ".githooks", true},
		{"nonexistent directories", filepath.Join(root, "missing"), "missing", true},
		{"symlink to the directory", "link", ".githooks", true},
		{"different directory", "other-hooks", ".githooks", false},
		{"different absolute directory", filepath.Join(root, "other-hooks"), ".githooks", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameHooksDirectory(root, tt.got, tt.want); got != tt.same {
				t.Fatalf("sameHooksDirectory(%q, %q) = %v, want %v", tt.got, tt.want, got, tt.same)
			}
		})
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

// gitTestHelper's env entries go last, so they override both the isolation
// defaults and the inherited environment (exec keeps the last value for a
// repeated key).
func gitTestHelper(t *testing.T, repoRoot string, env ...string) func(args ...string) {
	t.Helper()
	return func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(append([]string{
			"HOME=" + repoRoot,
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
		}, os.Environ()...), env...)
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

// catch-matrix: commit | caught | commit message doesn't match commit.pattern
func TestCheckCommit(t *testing.T) {
	t.Run("no message file", func(t *testing.T) {
		directory := t.TempDir()
		r := checkCommit(directory, "", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("pattern not configured", func(t *testing.T) {
		directory := t.TempDir()
		r := checkCommit(directory, "msg.txt", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '['\n")
		r := checkCommit(directory, "msg.txt", loadTestManifest(t, directory))
		if len(r.findings) != 1 || r.findings[0].check != "commit" {
			t.Fatalf("result = %+v, want commit finding", r)
		}
	})

	t.Run("unreadable message file", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat'\n")
		r := checkCommit(directory, filepath.Join(directory, "missing.txt"), loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("matches subject only", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, directory, "msg.txt", "feat: add things\n\nbody line\n")
		r := checkCommit(directory, msg, loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("does not match", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n")
		msg := writeTestFile(t, directory, "msg.txt", "fix: nope\n")
		r := checkCommit(directory, msg, loadTestManifest(t, directory))
		if len(r.findings) != 1 {
			t.Fatalf("result = %+v, want one finding", r)
		}
		f := r.findings[0]
		if f.check != "commit" || f.level != "error" || f.file != msg || f.line != 1 {
			t.Fatalf("finding = %+v", f)
		}
		if strings.Contains(f.message, "default merge message") {
			t.Fatalf("message = %q, want no merge hint for a non-merge subject", f.message)
		}
	})

	t.Run("git default merge message gets a hint", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^(merge: dev|feat: .+)'\n")
		for _, subject := range []string{
			"Merge branch 'dev'",
			"Merge remote-tracking branch 'origin/dev'",
			"Merge tag 'v1.0.0'",
			"Merge commit 'abc123'",
		} {
			msg := writeTestFile(t, directory, "msg.txt", subject+"\n")
			r := checkCommit(directory, msg, loadTestManifest(t, directory))
			if len(r.findings) != 1 {
				t.Fatalf("%q: result = %+v, want one finding", subject, r)
			}
			for _, want := range []string{subject, "default merge message", "-m '<a subject matching commit.pattern>'"} {
				if !strings.Contains(r.findings[0].message, want) {
					t.Fatalf("message = %q, want it to contain %q", r.findings[0].message, want)
				}
			}
		}
	})

	t.Run("merge message the pattern allows", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "commit:\n  pattern: '^(Merge branch |feat: .+)'\n")
		msg := writeTestFile(t, directory, "msg.txt", "Merge branch 'dev'\n")
		r := checkCommit(directory, msg, loadTestManifest(t, directory))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})
}

// catch-matrix: core | caught | installed cinch doesn't match an exact pin or is below a floor
func TestCheckPin(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		directory := t.TempDir()
		r := checkPin("1.0.0", loadTestManifest(t, directory))
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
		for _, want := range []string{"'>=1.0.0'", "any"} {
			if !strings.Contains(r.noOp, want) {
				t.Fatalf("noOp = %q, want it to contain %q", r.noOp, want)
			}
		}
	})

	t.Run("any disables the check", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: any\n")
		r := checkPin("1.0.0", loadTestManifest(t, directory))
		if !strings.Contains(r.noOp, "disabled") || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want a disabled noOp", r)
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

// cinch:rule CINCH-001
// catch-matrix: generated | caught | hand-edited rendered file (CINCH-001)
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

// catch-matrix: generated | caught | rendered file deleted
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

// cinch:rule CINCH-002
// catch-matrix: generated | caught | generated file the current render no longer produces (CINCH-002)
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
		if strings.Contains(f.file, "orphan-leftover.md") && strings.Contains(f.message, "orphaned generated file, no longer produced by cinch render — run 'cinch render' to remove it") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want an 'orphaned generated file' finding for orphan-leftover.md", result.findings)
	}
}

// squashTestRepo is a repository on branch with git mid squash-merge: git
// writes SQUASH_MSG into the git dir before it opens the editor, and the
// commit-msg hook runs while that file exists.
func squashTestRepo(t *testing.T, repoRoot, branch string) {
	t.Helper()
	git := initTestGitRepo(t, repoRoot)
	git("symbolic-ref", "HEAD", "refs/heads/"+branch)
	writeTestFile(t, filepath.Join(repoRoot, ".git"), "SQUASH_MSG", "Squashed commit of the following:\n")
}

// cinch:rule CINCH-015
// catch-matrix: commit | caught | squash-merge subject that doesn't match commit.squash.<branch> (CINCH-015)
func TestCheckCommitSquash(t *testing.T) {
	const squashManifest = "commit:\n  pattern: '^(feat|fix): .+'\n  squash:\n    dev: '^merge: (feature|fix)/.+'\n    main: '^merge: hotfix/.+'\n"
	cases := []struct {
		name, branch, manifest, subject string
		wantFinding                     bool
		wantIn                          string
	}{
		{"squash onto dev with a merge subject", "dev", squashManifest, "merge: feature/x\n", false, ""},
		{"squash onto dev with an ordinary subject", "dev", squashManifest, "feat: x\n", true, "squash-merge onto dev: commit message does not match commit.squash.dev"},
		{"squash onto main checks its own entry", "main", squashManifest, "merge: feature/x\n", true, "commit.squash.main"},
		{"squash onto an unlisted branch falls back to commit.pattern", "topic", squashManifest, "merge: feature/x\n", true, "commit.pattern"},
		{"squash onto an unlisted branch, ordinary subject", "topic", squashManifest, "feat: x\n", false, ""},
		{"no squash key: a squash is an ordinary commit", "dev", "commit:\n  pattern: '^(feat|fix): .+'\n", "merge: feature/x\n", true, "commit.pattern"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			squashTestRepo(t, repoRoot, tc.branch)
			writeTestFile(t, repoRoot, "cinch.yml", tc.manifest)
			msg := writeTestFile(t, repoRoot, "msg.txt", tc.subject)
			r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
			if (len(r.findings) == 1) != tc.wantFinding || r.noOp != "" {
				t.Fatalf("result = %+v, want finding=%v", r, tc.wantFinding)
			}
			if tc.wantFinding && !strings.Contains(r.findings[0].message, tc.wantIn) {
				t.Fatalf("message = %q, want it to contain %q", r.findings[0].message, tc.wantIn)
			}
		})
	}

	t.Run("squash entries are not consulted without SQUASH_MSG", func(t *testing.T) {
		repoRoot := t.TempDir()
		git := initTestGitRepo(t, repoRoot)
		git("symbolic-ref", "HEAD", "refs/heads/dev")
		writeTestFile(t, repoRoot, "cinch.yml", squashManifest)
		msg := writeTestFile(t, repoRoot, "msg.txt", "merge: feature/x\n")
		r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
		if len(r.findings) != 1 || !strings.Contains(r.findings[0].message, "commit.pattern") {
			t.Fatalf("result = %+v, want a commit.pattern finding", r)
		}
	})
}

// mergeTestRepo is a repository with git mid merge: MERGE_HEAD names the
// commit being merged until the merge commit is made.
func mergeTestRepo(t *testing.T, repoRoot string) {
	t.Helper()
	git := initTestGitRepo(t, repoRoot)
	writeTestFile(t, repoRoot, "a.txt", "a\n")
	git("add", "a.txt")
	git("commit", "-q", "-m", "feat: seed")
	writeTestFile(t, filepath.Join(repoRoot, ".git"), "MERGE_HEAD", "0000000000000000000000000000000000000000\n")
}

// cinch:rule CINCH-016
// catch-matrix: commit | not a defect | git's default merge subject on a real merge, with commit.merge: exempt (CINCH-016)
func TestCheckCommitMerge(t *testing.T) {
	cases := []struct {
		name, manifest, subject string
		wantNoOp                bool
		wantFinding             bool
		wantIn                  string
	}{
		{"exempt passes git's default subject unchecked", "commit:\n  pattern: '^feat: .+'\n  merge: exempt\n", "Merge branch 'dev'\n", true, false, ""},
		{"a merge pattern replaces commit.pattern", "commit:\n  pattern: '^feat: .+'\n  merge: '^merge: dev$'\n", "merge: dev\n", false, false, ""},
		{"a merge pattern that doesn't match", "commit:\n  pattern: '^feat: .+'\n  merge: '^merge: dev$'\n", "Merge branch 'dev'\n", false, true, "merge commit: commit message does not match commit.merge"},
		{"no merge key: commit.pattern applies as before", "commit:\n  pattern: '^feat: .+'\n", "Merge branch 'dev'\n", false, true, "default merge message"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			mergeTestRepo(t, repoRoot)
			writeTestFile(t, repoRoot, "cinch.yml", tc.manifest)
			msg := writeTestFile(t, repoRoot, "msg.txt", tc.subject)
			r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
			if (r.noOp != "") != tc.wantNoOp || (len(r.findings) == 1) != tc.wantFinding {
				t.Fatalf("result = %+v, want noOp=%v finding=%v", r, tc.wantNoOp, tc.wantFinding)
			}
			if tc.wantFinding && !strings.Contains(r.findings[0].message, tc.wantIn) {
				t.Fatalf("message = %q, want it to contain %q", r.findings[0].message, tc.wantIn)
			}
		})
	}

	t.Run("commit.merge is ignored outside a merge", func(t *testing.T) {
		repoRoot := t.TempDir()
		initTestGitRepo(t, repoRoot)
		writeTestFile(t, repoRoot, "cinch.yml", "commit:\n  pattern: '^feat: .+'\n  merge: exempt\n")
		msg := writeTestFile(t, repoRoot, "msg.txt", "Merge branch 'dev'\n")
		r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
		if len(r.findings) != 1 || r.noOp != "" {
			t.Fatalf("result = %+v, want a commit.pattern finding", r)
		}
	})
}

// cinch:rule CINCH-020
// catch-matrix: commit | caught | any commit made mid merge, with commit.merge: reject (CINCH-020)
func TestCheckCommitMergeReject(t *testing.T) {
	const rejectManifest = "commit:\n  pattern: '^(feat|fix): .+'\n  merge: reject\n"
	cases := []struct {
		name, subject string
	}{
		{"a subject that matches commit.pattern is still rejected", "feat: x\n"},
		{"git's default merge subject is rejected", "Merge branch 'dev'\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			mergeTestRepo(t, repoRoot)
			writeTestFile(t, repoRoot, "cinch.yml", rejectManifest)
			msg := writeTestFile(t, repoRoot, "msg.txt", tc.subject)
			r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
			if len(r.findings) != 1 || r.noOp != "" {
				t.Fatalf("result = %+v, want one finding", r)
			}
			f := r.findings[0]
			if f.level != "error" || f.check != "commit" || !strings.Contains(f.message, "make merge") {
				t.Fatalf("finding = %+v, want an error naming make merge", f)
			}
		})
	}

	t.Run("reject is ignored outside a merge", func(t *testing.T) {
		for subject, wantFinding := range map[string]bool{"feat: x\n": false, "Merge branch 'dev'\n": true} {
			repoRoot := t.TempDir()
			initTestGitRepo(t, repoRoot)
			writeTestFile(t, repoRoot, "cinch.yml", rejectManifest)
			msg := writeTestFile(t, repoRoot, "msg.txt", subject)
			r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
			if (len(r.findings) == 1) != wantFinding || r.noOp != "" {
				t.Fatalf("subject %q: result = %+v, want finding=%v", subject, r, wantFinding)
			}
			if wantFinding && !strings.Contains(r.findings[0].message, "commit.pattern") {
				t.Fatalf("subject %q: message = %q, want a commit.pattern finding", subject, r.findings[0].message)
			}
		}
	})

	t.Run("a squash is unaffected", func(t *testing.T) {
		repoRoot := t.TempDir()
		squashTestRepo(t, repoRoot, "dev")
		writeTestFile(t, repoRoot, "cinch.yml", rejectManifest)
		msg := writeTestFile(t, repoRoot, "msg.txt", "feat: x\n")
		r := checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want a pass", r)
		}
	})
}

// protectTestRepo is a repository on branch with one seed commit, so HEAD
// resolves; the caller adds SQUASH_MSG or MERGE_HEAD as needed.
func protectTestRepo(t *testing.T, repoRoot, branch string) func(args ...string) {
	t.Helper()
	git := initTestGitRepo(t, repoRoot)
	git("symbolic-ref", "HEAD", "refs/heads/"+branch)
	writeTestFile(t, repoRoot, "a.txt", "a\n")
	git("add", "a.txt")
	git("commit", "-q", "-m", "feat: seed")
	return git
}

// cinch:rule CINCH-021
// catch-matrix: commit | caught | plain commit on a branch listed in commit.protect (CINCH-021)
func TestCheckCommitProtect(t *testing.T) {
	const protectManifest = "commit:\n  protect: [main]\n"
	const patternManifest = "commit:\n  pattern: '^(feat|fix): .+'\n  protect: [main]\n"
	check := func(t *testing.T, repoRoot, manifest, subject string) checkResult {
		t.Helper()
		writeTestFile(t, repoRoot, "cinch.yml", manifest)
		msg := writeTestFile(t, repoRoot, "msg.txt", subject)
		return checkCommit(repoRoot, msg, loadTestManifest(t, repoRoot))
	}
	wantPass := func(t *testing.T, r checkResult) {
		t.Helper()
		if len(r.findings) != 0 {
			t.Fatalf("result = %+v, want no findings", r)
		}
	}

	t.Run("a plain commit on a protected branch fails and names make merge", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "main")
		r := check(t, repoRoot, patternManifest, "feat: x\n")
		if len(r.findings) != 1 || r.noOp != "" {
			t.Fatalf("result = %+v, want one finding", r)
		}
		f := r.findings[0]
		if f.level != "error" || f.check != "commit" || f.line != 1 {
			t.Fatalf("finding = %+v, want an error on line 1 of the commit check", f)
		}
		for _, want := range []string{"protected branch main", "commit.protect", "make merge"} {
			if !strings.Contains(f.message, want) {
				t.Fatalf("message = %q, want it to contain %q", f.message, want)
			}
		}
	})

	t.Run("the guard applies without commit.pattern", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "main")
		r := check(t, repoRoot, protectManifest, "anything\n")
		if len(r.findings) != 1 || !strings.Contains(r.findings[0].message, "make merge") {
			t.Fatalf("result = %+v, want the protect finding", r)
		}
	})

	t.Run("a squash commit on a protected branch passes", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "main")
		writeTestFile(t, filepath.Join(repoRoot, ".git"), "SQUASH_MSG", "Squashed commit of the following:\n")
		wantPass(t, check(t, repoRoot, patternManifest, "feat: x\n"))
	})

	for _, tc := range []struct{ name, manifest string }{
		{"commit.merge unset", protectManifest},
		{"commit.merge exempt", "commit:\n  protect: [main]\n  merge: exempt\n"},
		{"commit.merge a pattern", "commit:\n  protect: [main]\n  merge: '^merge: .+'\n"},
	} {
		t.Run("a merge commit on a protected branch passes with "+tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			protectTestRepo(t, repoRoot, "main")
			writeTestFile(t, filepath.Join(repoRoot, ".git"), "MERGE_HEAD", "0000000000000000000000000000000000000000\n")
			wantPass(t, check(t, repoRoot, tc.manifest, "merge: dev\n"))
		})
	}

	t.Run("commit.merge reject still fails a merge on a protected branch", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "main")
		writeTestFile(t, filepath.Join(repoRoot, ".git"), "MERGE_HEAD", "0000000000000000000000000000000000000000\n")
		r := check(t, repoRoot, "commit:\n  protect: [main]\n  merge: reject\n", "merge: dev\n")
		if len(r.findings) != 1 || !strings.Contains(r.findings[0].message, "commit.merge is reject") {
			t.Fatalf("result = %+v, want the CINCH-020 finding", r)
		}
	})

	t.Run("a commit on an unlisted branch passes", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "topic")
		wantPass(t, check(t, repoRoot, patternManifest, "feat: x\n"))
	})

	t.Run("the first commit on an unborn protected branch passes", func(t *testing.T) {
		repoRoot := t.TempDir()
		git := initTestGitRepo(t, repoRoot)
		git("symbolic-ref", "HEAD", "refs/heads/main")
		wantPass(t, check(t, repoRoot, patternManifest, "feat: x\n"))
	})

	t.Run("a commit on a detached HEAD passes", func(t *testing.T) {
		repoRoot := t.TempDir()
		git := protectTestRepo(t, repoRoot, "main")
		git("checkout", "-q", "--detach")
		wantPass(t, check(t, repoRoot, patternManifest, "feat: x\n"))
	})

	t.Run("without the key nothing changes", func(t *testing.T) {
		repoRoot := t.TempDir()
		protectTestRepo(t, repoRoot, "main")
		wantPass(t, check(t, repoRoot, "commit:\n  pattern: '^(feat|fix): .+'\n", "feat: x\n"))
		r := check(t, repoRoot, "commit:\n  pattern: '^(feat|fix): .+'\n", "oops\n")
		if len(r.findings) != 1 || !strings.Contains(r.findings[0].message, "commit.pattern") {
			t.Fatalf("result = %+v, want the commit.pattern finding", r)
		}
	})
}
