package cinch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// hookTestRepo is stagedTestRepo with the given cinch.yml committed, so the
// tree starts clean and a test dirties it deliberately.
func hookTestRepo(t *testing.T, root, manifest string) func(args ...string) {
	t.Helper()
	git := stagedTestRepo(t, root)
	writeTestFile(t, root, "cinch.yml", manifest)
	git("add", "cinch.yml")
	git("commit", "-q", "-m", "manifest")
	return git
}

func chdir(t *testing.T, directory string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
}

func runCheck(t *testing.T, changed bool) (code int, stdout, stderr string) {
	t.Helper()
	stdout = captureStdout(t, func() {
		stderr = captureStderr(t, func() { code = CmdCheck("", changed, false, nil) })
	})
	return code, stdout, stderr
}

func runPreCommitHook(t *testing.T, root string) (code int, stderr string) {
	t.Helper()
	captureStdout(t, func() {
		stderr = ansiPattern.ReplaceAllString(captureStderr(t, func() { code = cmdHookPreCommit(root) }), "")
	})
	return code, stderr
}

const (
	twoScriptManifest = "hooks:\n  pre-commit:\n    first:\n      run: exit 2\n    second:\n      run: touch second.ran\n"
	oneScriptManifest = "hooks:\n  pre-commit:\n    only:\n      run: 'true'\n"
)

// cinch:rule CINCH-012
// catch-matrix: rules | caught | staged rule whose only marker is an unstaged edit, in a hook script's plain 'cinch check' (CINCH-012)
func TestPlainCheckInsidePreCommitHookChecksStagedContent(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	stageRuleHiddenByUnstagedMarker(t, root, git)
	chdir(t, root)

	t.Setenv("CINCH_HOOK", "")
	if got, _, _ := runCheck(t, false); got != 0 {
		t.Fatalf("CmdCheck outside a hook = %d, want 0 (the working tree holds the marker)", got)
	}

	t.Setenv("CINCH_HOOK", "pre-commit")
	got, stdout, stderr := runCheck(t, false)
	if got != 1 {
		t.Fatalf("CmdCheck inside the pre-commit hook = %d, want 1 (the staged AUTH-005 has no marker)\nstdout: %s", got, stdout)
	}
	if !strings.Contains(stderr, "staged") {
		t.Fatalf("stderr = %q, want one line saying the check uses the staged content", stderr)
	}
}

// cinch:rule CINCH-012
func TestHookEnvironmentLeavesOtherChecksAlone(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	stageRuleHiddenByUnstagedMarker(t, root, git)
	chdir(t, root)

	t.Run("--changed stays on the working tree", func(t *testing.T) {
		t.Setenv("CINCH_HOOK", "pre-commit")
		got, _, stderr := runCheck(t, true)
		if got != 0 {
			t.Fatalf("CmdCheck --changed = %d, want 0 (the working tree holds the marker)", got)
		}
		if strings.Contains(stderr, "staged") {
			t.Fatalf("stderr = %q, want no staged-content notice for --changed", stderr)
		}
	})

	t.Run("other events don't switch to staged", func(t *testing.T) {
		t.Setenv("CINCH_HOOK", "commit-msg")
		if got, _, _ := runCheck(t, false); got != 0 {
			t.Fatalf("CmdCheck inside commit-msg = %d, want 0", got)
		}
	})
}

// cinch:rule CINCH-012
func TestHookScriptsSeeTheEvent(t *testing.T) {
	root := t.TempDir()
	hookTestRepo(t, root, "hooks:\n  pre-commit-checks: []\n  pre-commit:\n    probe:\n      run: printf %s \"$CINCH_HOOK\" > event.out\n")

	if code, _ := runPreCommitHook(t, root); code != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(root, "event.out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "pre-commit" {
		t.Fatalf("script saw CINCH_HOOK=%q, want pre-commit", got)
	}
}

func TestPreCommitHookWarnsWhenTreeDiffersFromCommit(t *testing.T) {
	root := t.TempDir()
	hookTestRepo(t, root, oneScriptManifest)
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Edited() {}\n")
	writeTestFile(t, root, "scratch.txt", "untracked\n")
	writeTestFile(t, root, "scratch2.txt", "untracked\n")

	code, stderr := runPreCommitHook(t, root)
	if code != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0 (a warning never blocks)\n%s", code, stderr)
	}
	want := "cinch: hook: warning: pre-commit scripts run in the working tree, which differs from this commit in 3 files (1 unstaged, 2 untracked)"
	if strings.Count(stderr, "warning:") != 1 || !strings.Contains(stderr, want+"\n") {
		t.Fatalf("stderr = %q, want exactly one line %q", stderr, want)
	}
}

func TestPreCommitHookDoesNotWarnOnCleanTree(t *testing.T) {
	root := t.TempDir()
	git := hookTestRepo(t, root, oneScriptManifest)
	// A staged change is what the commit contains; it doesn't make the tree differ.
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Staged() {}\n")
	git("add", "internal/auth.go")

	code, stderr := runPreCommitHook(t, root)
	if code != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0\n%s", code, stderr)
	}
	if strings.Contains(stderr, "warning") {
		t.Fatalf("stderr = %q, want no warning on a tree that matches the index", stderr)
	}
}

func TestPreCommitHookDoesNotWarnWhenNoScriptRuns(t *testing.T) {
	root := t.TempDir()
	hookTestRepo(t, root, "hooks:\n  pre-commit:\n    docs-only:\n      run: 'true'\n      when: [no-such-dir/]\n")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Edited() {}\n")
	writeTestFile(t, root, "other.txt", "x\n")
	git := gitTestHelper(t, root)
	git("add", "other.txt")

	code, stderr := runPreCommitHook(t, root)
	if code != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0\n%s", code, stderr)
	}
	if strings.Contains(stderr, "warning") {
		t.Fatalf("stderr = %q, want no warning when no script passes its when filter", stderr)
	}
}

// cinch:rule CINCH-013
func TestFailingHookStepDoesNotStopLaterStepsAndSummaryNamesEach(t *testing.T) {
	root := t.TempDir()
	hookTestRepo(t, root, twoScriptManifest)

	code, stderr := runPreCommitHook(t, root)
	if code != 1 {
		t.Fatalf("cmdHookPreCommit = %d, want 1 (the first step failed)", code)
	}
	if _, err := os.Stat(filepath.Join(root, "second.ran")); err != nil {
		t.Fatalf("the second step did not run after the first failed: %v", err)
	}
	if !strings.Contains(stderr, "pre-commit: first failed: exit status 2") {
		t.Fatalf("stderr = %q, want the failure reported where it happened", stderr)
	}

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	got := strings.Join(lines[len(lines)-4:], "\n")
	want := strings.Join([]string{
		"cinch: hook: pre-commit: 1 of 3 steps failed",
		"  checks  ok",
		"  first   failed (exit status 2)",
		"  second  ok",
	}, "\n")
	if got != want {
		t.Fatalf("last stderr lines:\n%s\nwant:\n%s", got, want)
	}
}

// cinch:rule CINCH-013
func TestHookSummaryCoversSkippedStepsAndOtherEvents(t *testing.T) {
	t.Run("a step skipped by when gets a row", func(t *testing.T) {
		root := t.TempDir()
		git := hookTestRepo(t, root, "hooks:\n  pre-commit:\n    docs-only:\n      run: 'true'\n      when: [no-such-dir/]\n    always:\n      run: 'true'\n")
		// With nothing staged every when filter matches, so stage something.
		writeTestFile(t, root, "other.txt", "x\n")
		git("add", "other.txt")
		code, stderr := runPreCommitHook(t, root)
		if code != 0 {
			t.Fatalf("cmdHookPreCommit = %d, want 0\n%s", code, stderr)
		}
		for _, want := range []string{"cinch: hook: pre-commit: 2 steps ok", "  docs-only  skipped", "  always     ok"} {
			if !strings.Contains(stderr, want+"\n") {
				t.Fatalf("stderr = %q, want line %q", stderr, want)
			}
		}
	})

	t.Run("post-commit has no checks row", func(t *testing.T) {
		root := t.TempDir()
		git := hookTestRepo(t, root, "hooks:\n  post-commit:\n    notify:\n      run: 'true'\n")
		git("commit", "-q", "--allow-empty", "-m", "another")
		var code int
		stderr := ansiPattern.ReplaceAllString(captureStderr(t, func() { code = cmdHookPostCommit(root) }), "")
		if code != 0 {
			t.Fatalf("cmdHookPostCommit = %d, want 0\n%s", code, stderr)
		}
		if !strings.Contains(stderr, "cinch: hook: post-commit: 1 step ok\n  notify  ok\n") || strings.Contains(stderr, "checks") {
			t.Fatalf("stderr = %q, want a summary with only the notify row", stderr)
		}
	})

	t.Run("no summary without a project script", func(t *testing.T) {
		root := t.TempDir()
		hookTestRepo(t, root, "hooks:\n  pre-commit-checks: []\n")
		_, stderr := runPreCommitHook(t, root)
		if strings.Contains(stderr, "steps") || strings.Contains(stderr, "step ") {
			t.Fatalf("stderr = %q, want no summary when no project script ran", stderr)
		}
	})
}
