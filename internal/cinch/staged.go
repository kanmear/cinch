package cinch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/output"
)

// stagedSnapshot returns checkRoots whose content is the index — what the
// commit being made will contain — instead of the working tree. cleanup is
// never nil, so a caller can defer it before looking at err.
//
// Every git call runs in repoRoot with the inherited environment, never with
// its working directory inside the snapshot: a hook may run with GIT_DIR and
// GIT_INDEX_FILE set (git commit -a and git commit <paths> stage into a
// temporary index and pass it through GIT_INDEX_FILE), and running in
// repoRoot is what makes git read that index instead of the default one.
//
// Cost: the slow path writes every tracked file into a temporary directory,
// so on a large repository each commit made with unstaged or untracked
// changes pays for one full checkout-index. The fast path skips that whenever
// the working tree already equals the index.
//
// One asymmetry between the two paths: gitignored files exist in the working
// tree but never in a snapshot, so a doc link that only resolves to an
// ignored file passes on the fast path and fails on the slow one. A fresh
// clone wouldn't have that file either, so the slow path's answer is the
// truer one.
func stagedSnapshot(repoRoot string) (roots checkRoots, cleanup func(), err error) {
	noCleanup := func() {}

	indexFiles, err := indexedFiles(repoRoot)
	if err != nil {
		return checkRoots{}, noCleanup, err
	}

	matches, err := workingTreeMatchesIndex(repoRoot)
	if err != nil {
		return checkRoots{}, noCleanup, err
	}
	if matches {
		return checkRoots{repoRoot: repoRoot, fsRoot: repoRoot, indexFiles: indexFiles}, noCleanup, nil
	}

	directory, err := os.MkdirTemp("", "cinch-staged-")
	if err != nil {
		return checkRoots{}, noCleanup, fmt.Errorf("creating index snapshot: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(directory) }
	prefix := "--prefix=" + directory + string(filepath.Separator)
	if out, err := gitutil.CombinedOutput(repoRoot, "checkout-index", "--all", "--force", prefix); err != nil {
		cleanup()
		return checkRoots{}, noCleanup, fmt.Errorf("git checkout-index failed: %s", strings.TrimSpace(string(out)))
	}
	return checkRoots{repoRoot: repoRoot, fsRoot: directory, indexFiles: indexFiles}, cleanup, nil
}

// indexedFiles never returns a nil list: to the marker scan, nil means "ask
// git yourself", and an empty index must still mean "scan nothing".
func indexedFiles(repoRoot string) ([]string, error) {
	files, err := gitutil.OutputPaths(repoRoot, "ls-files", "--cached")
	if err != nil {
		return nil, fmt.Errorf("git ls-files --cached failed: %w", err)
	}
	if files == nil {
		files = []string{}
	}
	return files, nil
}

// workingTreeMatchesIndex reports whether reading the working tree would see
// exactly the index: no unstaged change to a tracked file and no untracked,
// non-ignored file.
func workingTreeMatchesIndex(repoRoot string) (bool, error) {
	err := gitutil.Run(repoRoot, "diff", "--quiet", "--no-ext-diff")
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return false, nil
	default:
		return false, fmt.Errorf("git diff --quiet failed: %w", err)
	}

	untracked, err := gitutil.OutputLines(repoRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, fmt.Errorf("git ls-files --others failed: %w", err)
	}
	return len(untracked) == 0, nil
}

// stagedPreCommitChecks is the pre-commit hook's check bundle, run against
// the index rather than the working tree.
func stagedPreCommitChecks(repoRoot string) int {
	roots, cleanup, err := stagedSnapshot(repoRoot)
	defer cleanup()
	if err != nil {
		return output.Fail("hook", err)
	}
	return preCommitChecks(roots)
}
