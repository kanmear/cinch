package cinch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/output"
)

// indexListing is the index as stagedSnapshot needs it: every path, the set
// the checks consult, and whether any entry is one git diff won't compare
// against the working tree.
type indexListing struct {
	files           []string
	set             *indexSet
	assumeUnchanged bool
	skipWorktree    bool
}

// indexSet answers whether a repo-relative, slash-separated path is something
// the commit contains: an indexed file, or a directory holding one. It is
// built once and never written to afterwards, because checks read it from
// concurrent goroutines.
type indexSet struct {
	files       map[string]bool
	directories map[string]bool
}

func newIndexSet(files []string) *indexSet {
	set := &indexSet{
		files:       make(map[string]bool, len(files)),
		directories: make(map[string]bool),
	}
	for _, file := range files {
		set.files[file] = true
		for directory := path.Dir(file); directory != "." && !set.directories[directory]; directory = path.Dir(directory) {
			set.directories[directory] = true
		}
	}
	return set
}

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
// so on a large repository each commit made with unstaged changes pays for
// one full checkout-index. The fast path skips that whenever no tracked file
// differs from the index.
//
// checkout-index applies the repository's checkout filters (git-lfs,
// git-crypt), so in an LFS repository the slow path could pull every LFS
// object through its smudge filter on each commit made with unstaged
// changes. This is untested, and no known consumer uses LFS. If one does,
// write the snapshot from raw blobs (git cat-file --batch) instead.
//
// The two paths give one answer because the checks read only indexed paths
// (checkRoots.inCommit), so a file the commit lacks, untracked or ignored,
// can't change a result however it sits on disk. The exception is a tracked
// symlink whose target isn't indexed: the fast path reads through it. Nothing
// reads through symlinks on purpose, and no known consumer relies on one.
//
// cinch.yml is read by path, not through the index set, so an untracked one
// would be read on the fast path; it rules that path out.
//
// git diff doesn't compare an assume-unchanged or skip-worktree entry
// against the working tree, so either one rules the fast path out: the
// file on disk may hold unstaged edits, or (in a sparse checkout) not exist
// at all. checkout-index skips skip-worktree entries unless told otherwise,
// and the flag that tells it is newer than anything else cinch asks of git,
// so it's passed only when one is present: an older git then fails only in
// a repository that needs it.
func stagedSnapshot(repoRoot string) (roots checkRoots, cleanup func(), err error) {
	index, err := readIndex(repoRoot)
	if err != nil {
		return checkRoots{}, func() {}, err
	}

	if !index.assumeUnchanged && !index.skipWorktree {
		unchanged, err := trackedFilesMatchIndex(repoRoot)
		if err != nil {
			return checkRoots{}, func() {}, err
		}
		if unchanged && manifestIsIndexedOrAbsent(repoRoot, index) {
			return checkRoots{repoRoot: repoRoot, fsRoot: repoRoot, indexFiles: index.files, index: index.set}, func() {}, nil
		}
	}
	return checkoutSnapshot(repoRoot, index)
}

// checkoutSnapshot writes the index into a temporary directory.
func checkoutSnapshot(repoRoot string, index indexListing) (roots checkRoots, cleanup func(), err error) {
	noCleanup := func() {}
	directory, err := os.MkdirTemp("", "cinch-staged-")
	if err != nil {
		return checkRoots{}, noCleanup, fmt.Errorf("creating index snapshot: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(directory) }
	args := []string{"checkout-index", "--all", "--force", "--prefix=" + directory + string(filepath.Separator)}
	if index.skipWorktree {
		args = append(args, "--ignore-skip-worktree-bits")
	}
	if out, err := gitutil.CombinedOutput(repoRoot, args...); err != nil {
		cleanup()
		return checkRoots{}, noCleanup, fmt.Errorf("git checkout-index failed: %s", strings.TrimSpace(string(out)))
	}
	return checkRoots{repoRoot: repoRoot, fsRoot: directory, indexFiles: index.files, index: index.set}, cleanup, nil
}

// readIndex lists the index with `git ls-files -v`, whose one-letter tag
// marks an assume-unchanged entry in lowercase and a skip-worktree one as S.
// files is never nil: to the marker scan, nil means "ask git yourself", and
// an empty index must still mean "scan nothing".
func readIndex(repoRoot string) (indexListing, error) {
	entries, err := gitutil.OutputPaths(repoRoot, "ls-files", "-v", "--cached")
	if err != nil {
		return indexListing{}, fmt.Errorf("git ls-files --cached failed: %w", err)
	}
	index := indexListing{files: make([]string, 0, len(entries))}
	for _, entry := range entries {
		tag, file, ok := strings.Cut(entry, " ")
		if !ok {
			return indexListing{}, fmt.Errorf("git ls-files -v: unexpected entry %q", entry)
		}
		index.files = append(index.files, file)
		if tag != strings.ToUpper(tag) {
			index.assumeUnchanged = true
		}
		if strings.EqualFold(tag, "S") {
			index.skipWorktree = true
		}
	}
	index.set = newIndexSet(index.files)
	return index, nil
}

// trackedFilesMatchIndex reports whether git diff finds no unstaged change to
// a tracked file. Untracked and ignored files don't count: the checks never
// read them.
func trackedFilesMatchIndex(repoRoot string) (bool, error) {
	err := gitutil.Run(repoRoot, "diff", "--quiet", "--no-ext-diff")
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return false, nil
	default:
		return false, fmt.Errorf("git diff --quiet failed: %w", err)
	}
}

// manifestIsIndexedOrAbsent is false for a cinch.yml on disk that the commit
// doesn't contain. git diff is clean for it, yet loadManifestOptional would
// read it in place.
func manifestIsIndexedOrAbsent(repoRoot string, index indexListing) bool {
	if index.set.files[manifestPath] {
		return true
	}
	_, err := os.Stat(filepath.Join(repoRoot, manifestPath))
	return err != nil
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
