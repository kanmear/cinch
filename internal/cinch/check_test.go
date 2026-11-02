package cinch

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// captureStderr temporarily redirects os.Stderr (which output.CheckStatus
// writes through) so a test can assert on which check status lines did or
// didn't print.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureFile(t, &os.Stderr, fn)
}

// captureStdout temporarily redirects os.Stdout, where runChecks prints its
// findings.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureFile(t, &os.Stdout, fn)
}

func captureFile(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := *target
	*target = w
	fn()
	*target = original
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, r); err != nil {
		t.Fatal(err)
	}
	return buffer.String()
}

// initTestGitRepo initializes a git repo with a committer identity set, so
// callers can immediately `git commit` without hitting user.email/user.name
// errors.
func initTestGitRepo(t *testing.T, repoRoot string) func(args ...string) {
	t.Helper()
	git := gitTestHelper(t, repoRoot)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	return git
}

// changedTestRepo commits an initial state, then leaves the working tree
// dirty with unstaged edits made by dirty — mirroring how a developer running
// `cinch check --changed` would find their tree.
func changedTestRepo(t *testing.T, repoRoot string, seed, dirty func()) {
	t.Helper()
	git := initTestGitRepo(t, repoRoot)
	seed()
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	dirty()
}

// TestHookChecksSeparate pins the pre-commit/commit-msg split: the static
// suite catches doc issues, while commit-msg checks only the subject pattern
// and is unaffected by broken docs.
func TestHookChecksSeparate(t *testing.T) {
	root := t.TempDir()

	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "[broken](docs.md)\n")
	writeTestFile(t, root, "cinch.yml", "commit:\n  pattern: '^docs: .+'\n")
	good := writeTestFile(t, root, "msg-good.txt", "docs: remove completed plan\n")
	bad := writeTestFile(t, root, "msg-bad.txt", "oops: not matching\n")

	if got := preCommitChecks(workingTreeRoots(root)); got != 1 {
		t.Fatalf("preCommitChecks = %d, want 1 (broken doc link)", got)
	}
	if got := commitMsgChecks(root, good); got != 0 {
		t.Fatalf("commitMsgChecks(matching) = %d, want 0 (broken docs must not leak in)", got)
	}
	if got := commitMsgChecks(root, bad); got != 1 {
		t.Fatalf("commitMsgChecks(non-matching) = %d, want 1", got)
	}
}

// TestPreCommitChecksHonorsManifestOverride pins hooks.pre-commit-checks:
// with "links" excluded from the configured baseline, a doc-link finding
// that would otherwise fail the hook must not surface. The baseline is
// narrowed to ["core"] specifically because core no-ops with no
// require.cinch set (checks_misc.go's checkPin), isolating "did the override
// take effect" from any other check's behavior in a minimal fixture.
func TestPreCommitChecksHonorsManifestOverride(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "[broken](missing.md)\n")
	writeTestFile(t, root, "cinch.yml", "hooks:\n  pre-commit-checks: [core]\n")

	if got := preCommitChecks(workingTreeRoots(root)); got != 0 {
		t.Fatalf("preCommitChecks = %d, want 0 (links excluded from the configured baseline)", got)
	}
}

// TestCommitMsgChecksHonorsManifestOverride mirrors the pre-commit case for
// hooks.commit-msg-checks: with "commit" excluded, a message that violates
// an active commit.pattern must not fail the hook.
func TestCommitMsgChecksHonorsManifestOverride(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "commit:\n  pattern: '^docs: .+'\nhooks:\n  commit-msg-checks: [core]\n")
	msg := writeTestFile(t, root, "msg.txt", "not-matching-at-all\n")

	if got := commitMsgChecks(root, msg); got != 0 {
		t.Fatalf("commitMsgChecks = %d, want 0 (commit check excluded from the configured baseline)", got)
	}
}

// TestPreCommitChecksExplicitEmptyRunsNothing pins that
// hooks.pre-commit-checks: [] turns the built-in checks off, rather than
// being treated like an absent key and falling back to the defaults.
func TestPreCommitChecksExplicitEmptyRunsNothing(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "[broken](missing.md)\n")
	writeTestFile(t, root, "cinch.yml", "hooks:\n  pre-commit-checks: []\n")

	var got int
	stdout := captureStdout(t, func() {
		got = preCommitChecks(workingTreeRoots(root))
	})
	if got != 0 {
		t.Fatalf("preCommitChecks = %d, want 0 (empty list disables the built-in checks)", got)
	}
	if strings.Contains(stdout, "missing.md") {
		t.Fatalf("a link finding printed despite pre-commit-checks: []:\n%s", stdout)
	}
}

// TestCommitMsgChecksExplicitEmptyRunsNothing is the commit-msg counterpart:
// with commit-msg-checks: [], a message violating an active commit.pattern
// must neither fail the hook nor print a commit finding.
func TestCommitMsgChecksExplicitEmptyRunsNothing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "cinch.yml", "commit:\n  pattern: '^docs: .+'\nhooks:\n  commit-msg-checks: []\n")
	msg := writeTestFile(t, root, "msg.txt", "not-matching-at-all\n")

	var got int
	stdout := captureStdout(t, func() {
		stderr := captureStderr(t, func() {
			got = commitMsgChecks(root, msg)
		})
		if strings.Contains(stderr, "commit") {
			t.Errorf("a commit status line printed despite commit-msg-checks: []:\n%s", stderr)
		}
	})
	if got != 0 {
		t.Fatalf("commitMsgChecks = %d, want 0 (empty list disables the built-in checks)", got)
	}
	if strings.Contains(stdout, "not-matching-at-all") || strings.Contains(stdout, "pattern") {
		t.Fatalf("a commit finding printed despite commit-msg-checks: []:\n%s", stdout)
	}
}

// TestCheckChangedFiltersLinksToChangedFiles pins the core --changed
// behavior: a pre-existing broken link in a doc that wasn't touched must not
// surface, but the same broken link does surface once its doc is the one
// with an unstaged edit.
func TestCheckChangedFiltersLinksToChangedFiles(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	changedTestRepo(t, root,
		func() {
			writeTestFile(t, docs, "broken.md", "# Broken\n\n[dead](nowhere.md)\n")
			writeTestFile(t, docs, "other.md", "# Other\n\nsome text\n")
		},
		func() {
			writeTestFile(t, docs, "other.md", "# Other\n\nsome text, edited\n")
		},
	)

	if got := runChecks(workingTreeRoots(root), "", true); got != 0 {
		t.Fatalf("runChecks(changed) = %d, want 0 (broken.md wasn't touched)", got)
	}

	writeTestFile(t, docs, "broken.md", "# Broken\n\n[dead](nowhere.md)\nedited too\n")
	if got := runChecks(workingTreeRoots(root), "", true); got != 1 {
		t.Fatalf("runChecks(changed) = %d, want 1 (broken.md is now the changed file)", got)
	}
}

// TestCheckChangedSkipsIrrelevantChecksVisibly pins that --changed accounts
// for every check it leaves out: when nothing under docsRoot changed, links
// and index print a skip line rather than vanishing from the run.
func TestCheckChangedSkipsIrrelevantChecksVisibly(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	changedTestRepo(t, root,
		func() {
			writeTestFile(t, docs, "a.md", "# A\n\ntext\n")
			writeTestFile(t, root, "other.txt", "unrelated\n")
		},
		func() {
			writeTestFile(t, root, "other.txt", "unrelated, edited\n")
		},
	)

	out := captureStderr(t, func() {
		if got := runChecks(workingTreeRoots(root), "", true); got != 0 {
			t.Fatalf("runChecks(changed) = %d, want 0", got)
		}
	})

	for _, name := range []string{"links", "index", "generated"} {
		want := name + ": skip: nothing relevant changed"
		if !strings.Contains(out, want) {
			t.Fatalf("stderr missing %q, want a skip line for the irrelevant check; stderr=%q", want, out)
		}
	}
	// commit is not a diff-scoping question, so --changed prints nothing for it.
	if strings.Contains(out, "commit:") {
		t.Fatalf("stderr contains %q, want commit left out without a MSGFILE; stderr=%q", "commit:", out)
	}
	// rules is deliberately NOT skipped here: rule markers can live
	// outside docsRoot, so any changed file at all keeps it relevant.
	if strings.Contains(out, "rules: skip: nothing relevant changed") {
		t.Fatalf("stderr = %q, want rules to stay relevant when any file changed", out)
	}
}

// TestCheckChangedLeavesOnlyExcludedChecksOut pins that the skip lines are
// for --changed alone: a check --only leaves out prints nothing.
func TestCheckChangedLeavesOnlyExcludedChecksOut(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	changedTestRepo(t, root,
		func() {
			writeTestFile(t, docs, "a.md", "# A\n\ntext\n")
			writeTestFile(t, root, "other.txt", "unrelated\n")
		},
		func() { writeTestFile(t, root, "other.txt", "unrelated, edited\n") },
	)

	out := captureStderr(t, func() { runChecks(workingTreeRoots(root), "", true, "links") })
	if out != "links: skip: nothing relevant changed\n" {
		t.Fatalf("stderr = %q, want only links' skip line", out)
	}
}

// TestCheckCommitSuppressedWithoutMessageFile pins commit's relevance
// predicate: it goes silent whenever no MSGFILE was given — on plain `cinch
// check` just as much as under --changed, since nobody hand-invokes `cinch
// check somefile.txt` in ordinary use — but still runs when one is provided,
// and says why it didn't when --only asked for it by name.
func TestCheckCommitSuppressedWithoutMessageFile(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	changedTestRepo(t, root,
		func() { writeTestFile(t, docs, "a.md", "# A\n\ntext\n") },
		func() { writeTestFile(t, docs, "a.md", "# A\n\ntext, edited\n") },
	)

	for _, changed := range []bool{false, true} {
		out := captureStderr(t, func() { runChecks(workingTreeRoots(root), "", changed) })
		if bytes.Contains([]byte(out), []byte("commit:")) {
			t.Fatalf("changed=%v: stderr contains %q, want commit suppressed with no MSGFILE; stderr=%q", changed, "commit:", out)
		}
	}

	for _, changed := range []bool{false, true} {
		out := captureStderr(t, func() { runChecks(workingTreeRoots(root), "", changed, "commit") })
		if !strings.Contains(out, "commit: skip: no commit message file given") {
			t.Fatalf("changed=%v: stderr = %q, want a skip line for an explicit --only commit with no MSGFILE", changed, out)
		}
	}

	msg := writeTestFile(t, root, "msg.txt", "docs: edit a.md\n")
	for _, changed := range []bool{false, true} {
		out := captureStderr(t, func() { runChecks(workingTreeRoots(root), msg, changed) })
		if !bytes.Contains([]byte(out), []byte("commit:")) {
			t.Fatalf("changed=%v: stderr missing %q, want commit to run when a MSGFILE is given; stderr=%q", changed, "commit:", out)
		}
	}
}

// TestCheckChangedGeneratedCoarseGate pins generated's coarse relevance: it
// only re-runs when the manifest or a rendered destination path changed, not
// on every unrelated edit — even though real drift may exist on disk either
// way, since checkGenerated always scans everything once it decides to run.
func TestCheckChangedGeneratedCoarseGate(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	changedTestRepo(t, root,
		func() {
			writeTestFile(t, root, "cinch.yml", "")
			writeTestFile(t, docs, "a.md", "# A\n\ntext\n")
		},
		func() {
			writeTestFile(t, docs, "a.md", "# A\n\ntext, edited\n")
		},
	)

	out := captureStderr(t, func() {
		runChecks(workingTreeRoots(root), "", true)
	})
	if !strings.Contains(out, "generated: skip: nothing relevant changed") {
		t.Fatalf("stderr = %q, want generated skipped (cinch.yml untouched)", out)
	}

	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
	out = captureStderr(t, func() {
		runChecks(workingTreeRoots(root), "", true)
	})
	if !bytes.Contains([]byte(out), []byte("generated:")) || strings.Contains(out, "generated: skip: nothing relevant changed") {
		t.Fatalf("stderr = %q, want generated to run (cinch.yml changed)", out)
	}
}

// TestCheckOnlyRestrictsCheckerSet exposes the pre-existing internal only
// mechanism: with --only links, no other check's status line should appear
// even when --changed is also active.
func TestCheckOnlyRestrictsCheckerSet(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	changedTestRepo(t, root,
		func() { writeTestFile(t, docs, "a.md", "# A\n\ntext\n") },
		func() { writeTestFile(t, docs, "a.md", "# A\n\ntext, edited\n") },
	)

	out := captureStderr(t, func() {
		runChecks(workingTreeRoots(root), "", true, "links", "index")
	})
	for _, name := range []string{"rules:", "generated:", "commit:", "core:", "hooks:"} {
		if bytes.Contains([]byte(out), []byte(name)) {
			t.Fatalf("stderr contains %q, want only links/index to run; stderr=%q", name, out)
		}
	}
	if !bytes.Contains([]byte(out), []byte("links:")) {
		t.Fatalf("stderr missing %q; stderr=%q", "links:", out)
	}
}

// TestRunChecksStatusLinesInFixedOrder pins the print order of the status
// lines: checks finish in scheduler order, so the same run repeated must still
// come out in checkLaunchOrder every time.
func TestRunChecksStatusLinesInFixedOrder(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "# A\n\ntext\n")
	msg := writeTestFile(t, root, "msg.txt", "docs: edit a.md\n")

	want := []string{"links", "rules", "index", "generated", "commit", "core", "hooks"}
	if !reflect.DeepEqual(checkLaunchOrder, want) {
		t.Fatalf("checkLaunchOrder = %v, want %v", checkLaunchOrder, want)
	}

	for run := 0; run < 20; run++ {
		out := captureStderr(t, func() { runChecks(workingTreeRoots(root), msg, false) })

		var got []string
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			prefix, _, _ := strings.Cut(line, ":")
			got = append(got, prefix)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: status line order = %v, want %v; stderr=%q", run, got, want, out)
		}
	}
}

// TestRunChecksOrdersFindingsLikeStatusLines pins that the finding details
// follow checkLaunchOrder, the order of the status lines above them, rather
// than the check names' alphabetical order (which would put core first).
func TestRunChecksOrdersFindingsLikeStatusLines(t *testing.T) {
	withVersion(t, "1.0.0")
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "[broken](missing.md)\n")
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '2.0.0'\n")

	var got int
	out := captureStdout(t, func() { got = runChecks(workingTreeRoots(root), "", false) })
	if got != 1 {
		t.Fatalf("runChecks = %d, want 1\n%s", got, out)
	}
	links, core := strings.Index(out, "links error"), strings.Index(out, "core error")
	if links < 0 || core < 0 || links > core {
		t.Fatalf("findings = %q, want the links finding before the core finding", out)
	}
}
