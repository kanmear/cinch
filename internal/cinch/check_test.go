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
func initTestGitRepo(t *testing.T, root string) func(args ...string) {
	t.Helper()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	return git
}

// changedTestRepo commits an initial state, then leaves the working tree
// dirty with unstaged edits made by dirty — mirroring how a developer running
// `cinch check --changed` would find their tree.
func changedTestRepo(t *testing.T, root string, seed, dirty func()) {
	t.Helper()
	git := initTestGitRepo(t, root)
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

// TestCheckChangedSuppressesIrrelevantCheckStatus is the "no unstaged files
// means no related output" behavior: when nothing under docsRoot changed,
// links/index must not print a status line at all, not even a skip line.
func TestCheckChangedSuppressesIrrelevantCheckStatus(t *testing.T) {
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

	for _, name := range []string{"links:", "index:", "commit:"} {
		if bytes.Contains([]byte(out), []byte(name)) {
			t.Fatalf("stderr contains %q, want it fully suppressed; stderr=%q", name, out)
		}
	}
	// rules is deliberately NOT suppressed here: rule markers can live
	// outside docsRoot, so any changed file at all keeps it relevant.
	if !bytes.Contains([]byte(out), []byte("rules:")) {
		t.Fatalf("stderr missing %q, want rules to stay relevant when any file changed; stderr=%q", "rules:", out)
	}
}

// TestCheckCommitSuppressedWithoutMessageFile pins commit's relevance
// predicate: it goes silent whenever no MSGFILE was given — on plain `cinch
// check` just as much as under --changed, since nobody hand-invokes `cinch
// check somefile.txt` in ordinary use — but still runs when one is provided.
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
	if bytes.Contains([]byte(out), []byte("generated:")) {
		t.Fatalf("stderr contains %q, want generated suppressed (cinch.yml untouched); stderr=%q", "generated:", out)
	}

	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
	out = captureStderr(t, func() {
		runChecks(workingTreeRoots(root), "", true)
	})
	if !bytes.Contains([]byte(out), []byte("generated:")) {
		t.Fatalf("stderr missing %q, want generated relevant (cinch.yml changed); stderr=%q", "generated:", out)
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
