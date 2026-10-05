package cinch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	// stagedMarker is split so these fixtures never read as markers when
	// cinch scans its own repository.
	stagedMarker      = "// cinch:" + "rule "
	stagedOneRuleDoc  = "# Auth rules\n\n1. **AUTH-001** sessions expire\n"
	stagedTwoRulesDoc = stagedOneRuleDoc + "2. **AUTH-005** tokens rotate\n"
	stagedMarkedCode  = "package auth\n\n" + stagedMarker + "AUTH-001\n"
)

// stagedTestRepo commits a clean corpus — one rule doc and the code marking
// its only rule — so a test can layer staged and unstaged edits on top.
func stagedTestRepo(t *testing.T, root string) func(args ...string) {
	t.Helper()
	git := initTestGitRepo(t, root)
	for _, directory := range []string{".docs", "internal"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedOneRuleDoc)
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode)
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	return git
}

// stageRuleHiddenByUnstagedMarker stages a new rule without its marker, then
// adds the marker to the working tree only: the commit would leave HEAD red,
// but the working tree looks clean.
func stageRuleHiddenByUnstagedMarker(t *testing.T, root string, git func(args ...string)) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc)
	git("add", ".docs/rules.md")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+stagedMarker+"AUTH-005\n")
}

// cinch:rule CINCH-006
func TestStagedChecksIgnoreUnstagedRuleEdit(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)

	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc)
	writeTestFile(t, root, "unrelated.txt", "staged change\n")
	git("add", "unrelated.txt")

	if got := preCommitChecks(workingTreeRoots(root)); got != 1 {
		t.Fatalf("working-tree preCommitChecks = %d, want 1 (the unstaged AUTH-005 is unmarked)", got)
	}
	if got := stagedPreCommitChecks(root); got != 0 {
		t.Fatalf("stagedPreCommitChecks = %d, want 0 (the unmarked rule isn't part of this commit)", got)
	}
}

// cinch:rule CINCH-006
func TestStagedChecksCatchRuleHiddenByUnstagedMarker(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	stageRuleHiddenByUnstagedMarker(t, root, git)

	if got := preCommitChecks(workingTreeRoots(root)); got != 0 {
		t.Fatalf("working-tree preCommitChecks = %d, want 0 (the marker is on disk)", got)
	}
	var got int
	out := captureStdout(t, func() { got = stagedPreCommitChecks(root) })
	if got != 1 {
		t.Fatalf("stagedPreCommitChecks = %d, want 1 (the staged rule has no staged marker)", got)
	}
	if !strings.Contains(out, "AUTH-005: no // cinch:rule marker") {
		t.Fatalf("output = %q, want a finding naming AUTH-005", out)
	}
}

// cinch:rule CINCH-006
func TestStagedChecksIgnoreMarkerInUntrackedFile(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc)
	git("add", ".docs/rules.md")
	writeTestFile(t, filepath.Join(root, "internal"), "rotate.go", "package auth\n\n"+stagedMarker+"AUTH-005\n")

	if got := preCommitChecks(workingTreeRoots(root)); got != 0 {
		t.Fatalf("working-tree preCommitChecks = %d, want 0 (the untracked file holds the marker)", got)
	}
	if got := stagedPreCommitChecks(root); got != 1 {
		t.Fatalf("stagedPreCommitChecks = %d, want 1 (an untracked file isn't part of the commit)", got)
	}
}

// git commit -a and git commit <paths> stage into a temporary index and hand
// it to the hook through GIT_INDEX_FILE; the snapshot must be built from it.
func TestStagedSnapshotHonorsGitIndexFile(t *testing.T) {
	root := t.TempDir()
	stagedTestRepo(t, root)

	alternateIndex := filepath.Join(t.TempDir(), "alternate-index")
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alternateIndex, data, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "internal"), "extra.go", "// staged in the alternate index\n")
	gitTestHelper(t, root, "GIT_INDEX_FILE="+alternateIndex)("add", "internal/extra.go")
	writeTestFile(t, filepath.Join(root, "internal"), "extra.go", "// unstaged edit\n")

	t.Setenv("GIT_INDEX_FILE", alternateIndex)
	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}

	if !slices.Contains(roots.indexFiles, "internal/extra.go") {
		t.Fatalf("indexFiles = %v, want internal/extra.go from the alternate index", roots.indexFiles)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (extra.go has an unstaged edit)")
	}
	if got := readTestFile(t, filepath.Join(roots.fsRoot, "internal"), "extra.go"); got != "// staged in the alternate index\n" {
		t.Fatalf("snapshot extra.go = %q, want the alternate index's content", got)
	}
}

func TestStagedChecksReportRepoRelativePaths(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	stageRuleHiddenByUnstagedMarker(t, root, git)

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	out := captureStdout(t, func() { stagedPreCommitChecks(".") })
	if !strings.Contains(out, " .docs/rules.md:4: AUTH-005") {
		t.Fatalf("output = %q, want the finding at .docs/rules.md:4", out)
	}
	if strings.Contains(out, "cinch-staged-") {
		t.Fatalf("output = %q, must not name the snapshot directory", out)
	}
}

func TestStagedSnapshotCleanup(t *testing.T) {
	root := t.TempDir()
	stagedTestRepo(t, root)
	writeTestFile(t, root, "untracked.txt", "forces a snapshot\n")

	roots, cleanup, err := stagedSnapshot(root)
	if err != nil {
		cleanup()
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(roots.fsRoot), "cinch-staged-") {
		t.Fatalf("fsRoot = %q, want a cinch-staged- snapshot directory", roots.fsRoot)
	}
	if _, err := os.Stat(filepath.Join(roots.fsRoot, ".docs", "rules.md")); err != nil {
		t.Fatalf("snapshot is missing a tracked file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(roots.fsRoot, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("snapshot holds an untracked file (stat err = %v)", err)
	}

	cleanup()
	if _, err := os.Stat(roots.fsRoot); !os.IsNotExist(err) {
		t.Fatalf("snapshot directory still exists after cleanup (stat err = %v)", err)
	}
}

func TestStagedChecksResolveSiblingRootAgainstRepo(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "sib")
	for _, directory := range []string{root, sibling} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitSibling := initTestGitRepo(t, sibling)
	writeTestFile(t, sibling, "rotate.go", "package rotate\n\n"+stagedMarker+"AUTH-005\n")
	gitSibling("add", "-A")
	gitSibling("commit", "-q", "-m", "seed")

	git := stagedTestRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "rules:\n  roots: [../sib]\n")
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc)
	git("add", "-A")
	writeTestFile(t, root, "untracked.txt", "forces a snapshot\n")

	// git commit -a hands the hook its index the same way, absolute.
	t.Setenv("GIT_INDEX_FILE", filepath.Join(root, ".git", "index"))

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	// An all-roots-missing skip would also exit 0, so check the report
	// itself: the sibling must have been found and scanned.
	report := checkRules(roots, filepath.Join(roots.fsRoot, defaultDocsPath), markerScanOptions{extraRoots: []string{"../sib"}})
	if report.skip != "" || len(report.findings) != 0 || report.rules != 2 {
		t.Fatalf("report = %+v, want 2 rules, no findings, no skip (AUTH-005 is marked in ../sib)", report)
	}
	var got int
	out := captureStdout(t, func() { got = stagedPreCommitChecks(root) })
	if got != 0 {
		t.Fatalf("stagedPreCommitChecks = %d, want 0\n%s", got, out)
	}
}

// git quotes a non-ASCII path in plain ls-files output; the index listing
// must still name the real file so its marker is found.
func TestStagedChecksFindMarkerInNonASCIIPath(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc)
	writeTestFile(t, filepath.Join(root, "internal"), "rötate.go", "package auth\n\n"+stagedMarker+"AUTH-005\n")
	git("add", "-A")

	if got := stagedPreCommitChecks(root); got != 0 {
		t.Fatalf("stagedPreCommitChecks = %d, want 0 (AUTH-005 is marked in internal/rötate.go)", got)
	}
}

func TestStagedChecksReadGeneratedFromIndex(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: .docs\n")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	rendered := filepath.Join(defaultDocsPath, workflowsSubdir, "docs-philosophy.md")
	path := filepath.Join(root, rendered)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, "hand edit\n"...), 0o644); err != nil {
		t.Fatal(err)
	}

	stagedGenerated := func() int {
		roots, cleanup, err := stagedSnapshot(root)
		defer cleanup()
		if err != nil {
			t.Fatalf("stagedSnapshot: %v", err)
		}
		return runChecks(roots, "", false, "generated")
	}

	if got := runChecks(workingTreeRoots(root), "", false, "generated"); got != 1 {
		t.Fatalf("working-tree generated = %d, want 1 (the rendered file is hand-edited on disk)", got)
	}
	if got := stagedGenerated(); got != 0 {
		t.Fatalf("staged generated = %d, want 0 (the hand edit is unstaged)", got)
	}
	git("add", rendered)
	if got := stagedGenerated(); got != 1 {
		t.Fatalf("staged generated = %d, want 1 (the hand edit is staged)", got)
	}
}

// The snapshot isn't a repository, so HEAD's cinch.yml — which names the
// docs root a rename left orphaned files under — has to come from repoRoot.
func TestStagedGeneratedFindsOrphansFromHeadManifest(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: .docs\n")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: docs2\n")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	// Render removes the old docs root's files; restore them, as a render by
	// an older cinch would have left them.
	git("checkout", "HEAD", "--", defaultDocsPath)
	git("add", "-A")
	writeTestFile(t, root, "untracked.txt", "forces a snapshot\n")

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot")
	}
	m, err := loadManifestOptional(roots.fsRoot)
	if err != nil {
		t.Fatal(err)
	}
	result := checkGenerated(roots, m, nil)
	orphan := filepath.Join(defaultDocsPath, workflowsSubdir, "docs-philosophy.md")
	for _, f := range result.findings {
		if f.file == orphan && strings.Contains(f.message, "orphaned") {
			return
		}
	}
	t.Fatalf("findings = %+v, want %s reported as orphaned", result.findings, orphan)
}

// With nothing unstaged or untracked, the snapshot is the working tree
// itself, and the result must be byte-for-byte what the old path gave.
func TestStagedSnapshotFastPathMatchesWorkingTree(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc+"\n[broken](missing.md)\n")
	git("commit", "-q", "-am", "unmarked rule and a broken link")

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot != root {
		t.Fatalf("fsRoot = %q, want repoRoot (the working tree equals the index)", roots.fsRoot)
	}

	var stagedCode, workingTreeCode int
	stagedOut := captureStdout(t, func() { stagedCode = preCommitChecks(roots) })
	workingTreeOut := captureStdout(t, func() { workingTreeCode = preCommitChecks(workingTreeRoots(root)) })
	if stagedCode != 1 || stagedCode != workingTreeCode {
		t.Fatalf("exit codes: staged %d, working tree %d, want both 1", stagedCode, workingTreeCode)
	}
	if stagedOut != workingTreeOut {
		t.Fatalf("staged output differs from the working tree's:\nstaged:\n%s\nworking tree:\n%s", stagedOut, workingTreeOut)
	}
}

// upgrade renders files it doesn't stage, so it must keep checking the
// working tree: checking the index would flag its own fresh render.
func TestCmdUpgradeChecksWorkingTreeNotIndex(t *testing.T) {
	withVersion(t, "1.2.4")
	withReleaseCheckDisabled(t)
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "require:\n  cinch: 1.2.3\n")
	staleDirectory := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	if err := os.MkdirAll(staleDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, staleDirectory, "docs-philosophy.md", "stale content\n")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	if code := CmdUpgrade(root); code != 0 {
		t.Fatalf("CmdUpgrade = %d, want 0 (the fresh render is on disk)", code)
	}

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if got := runChecks(roots, "", false, "generated"); got != 1 {
		t.Fatalf("staged generated = %d, want 1 — the index still holds the stale render, so this test proves upgrade read the working tree", got)
	}
}
