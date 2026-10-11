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
// catch-matrix: rules | not a defect | unstaged edit to a rule doc during a commit (CINCH-006)
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
// catch-matrix: rules | caught | staged rule whose only marker is an unstaged edit (CINCH-006)
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
// catch-matrix: rules | caught | staged rule whose only marker is in an untracked file (CINCH-006)
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

// cinch:rule CINCH-006
// catch-matrix: rules | caught | staged rule whose only marker is an unstaged edit to an assume-unchanged file (CINCH-006)
func TestStagedChecksCatchMarkerHiddenByAssumeUnchanged(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	// git diff doesn't compare an assume-unchanged file, so the unstaged
	// marker below is invisible to it; the working tree only looks clean.
	git("update-index", "--assume-unchanged", "internal/auth.go")
	stageRuleHiddenByUnstagedMarker(t, root, git)

	var got int
	out := captureStdout(t, func() { got = stagedPreCommitChecks(root) })
	if got != 1 {
		t.Fatalf("stagedPreCommitChecks = %d, want 1 (the staged rule has no staged marker)\n%s", got, out)
	}
	if !strings.Contains(out, "AUTH-005: no // cinch:rule marker") {
		t.Fatalf("output = %q, want a finding naming AUTH-005", out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | marker in a skip-worktree file a sparse checkout left off disk (CINCH-006)
func TestStagedChecksFindMarkerInSkipWorktreeFile(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	// A sparse checkout leaves a skip-worktree file off disk; the commit
	// still contains it, so its marker still counts.
	git("update-index", "--skip-worktree", "internal/auth.go")
	if err := os.Remove(filepath.Join(root, "internal", "auth.go")); err != nil {
		t.Fatal(err)
	}

	var got int
	out := captureStdout(t, func() { got = stagedPreCommitChecks(root) })
	if got != 0 {
		t.Fatalf("stagedPreCommitChecks = %d, want 0 (AUTH-001 is marked in the index's internal/auth.go)\n%s", got, out)
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

// Impact is advisory, but it must describe the commit, not the working tree:
// a rule that exists only in unstaged edits is not part of what is committed.
func TestImpactReadsStagedSnapshot(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	ownedDoc := "---\nowns:\n  - internal\nrule_prefix: AUTH-\n---\n" + stagedOneRuleDoc
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", ownedDoc)
	git("add", "-A")
	git("commit", "-q", "-m", "own internal")

	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Staged() {}\n")
	git("add", "internal/auth.go")
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", ownedDoc+"2. **AUTH-002** tokens rotate\n")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go",
		stagedMarkedCode+"\nfunc Staged() {}\n"+stagedMarker+"AUTH-002\n")

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (the doc and code have unstaged edits)")
	}
	docsRoot, err := ResolveDocsRoot(roots.fsRoot)
	if err != nil {
		t.Fatalf("ResolveDocsRoot: %v", err)
	}
	hits, err := buildImpact(roots, docsRoot, []string{"internal/auth.go"}, nil)
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}

	var sawMarker, sawOwns bool
	for _, h := range hits {
		if h.ruleID == "AUTH-002" || slices.Contains(h.ruleIDs, "AUTH-002") {
			t.Fatalf("hit %+v names AUTH-002, which exists only in unstaged edits", h)
		}
		switch h.kind {
		case "marker":
			sawMarker = h.ruleID == "AUTH-001" && len(h.markers) == 1 &&
				h.markers[0] == markerLoc{file: "internal/auth.go", line: 3}
		case "owns":
			sawOwns = h.doc == ".docs/rules.md" && slices.Equal(h.ruleIDs, []string{"AUTH-001"})
		}
	}
	if !sawMarker || !sawOwns {
		t.Fatalf("hits = %+v, want an AUTH-001 marker hit at internal/auth.go:3 and an owns hit on .docs/rules.md", hits)
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
	writeTestFile(t, root, "untracked.txt", "not in the commit\n")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Unstaged() {}\n")

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
	if got := readTestFile(t, filepath.Join(roots.fsRoot, "internal"), "auth.go"); got != stagedMarkedCode {
		t.Fatalf("snapshot auth.go = %q, want the staged content, not the unstaged edit", got)
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
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Unstaged() {}\n")

	// git commit -a hands the hook its index the same way, absolute.
	t.Setenv("GIT_INDEX_FILE", filepath.Join(root, ".git", "index"))

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (auth.go has an unstaged edit)")
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
	writeTestFile(t, root, "notes.txt", "seed\n")
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
	writeTestFile(t, root, "notes.txt", "unstaged edit\n")

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (notes.txt has an unstaged edit)")
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

// With nothing unstaged, the snapshot is the working tree
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

// outOfRepoLinkRepo builds a repository whose rule doc links to a file that
// sits next to the repository, not in it, and leaves an unstaged edit to a
// tracked file so the snapshot path is taken.
func outOfRepoLinkRepo(t *testing.T, siblingFile string) (root string) {
	t.Helper()
	parent := t.TempDir()
	root = filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "esc-sibling")
	for _, directory := range []string{root, sibling} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, sibling, "README.md", "# Sibling\n")

	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedOneRuleDoc+"\n[sib](../../"+siblingFile+")\n")
	git("add", "-A")
	git("commit", "-q", "-m", "link out of the repository")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"\nfunc Unstaged() {}\n")
	return root
}

// cinch:rule CINCH-006
// catch-matrix: links | not a defect | link to a file outside the repository, checked from an index snapshot (CINCH-006)
func TestStagedLinksResolveOutOfRepoTargetFromRepo(t *testing.T) {
	root := outOfRepoLinkRepo(t, "esc-sibling/README.md")

	if got := runChecks(workingTreeRoots(root), "", false, "links"); got != 0 {
		t.Fatalf("working-tree links = %d, want 0 (the sibling file exists)", got)
	}
	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (auth.go has an unstaged edit)")
	}
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "links") })
	if got != 0 {
		t.Fatalf("staged links = %d, want 0 (a link out of the repository resolves from the repository, not the snapshot)\n%s", got, out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: links | caught | link to a missing file outside the repository, on both paths (CINCH-006)
func TestStagedLinksCatchMissingOutOfRepoTarget(t *testing.T) {
	root := outOfRepoLinkRepo(t, "esc-missing/README.md")

	var workingTreeCode int
	workingTreeOut := captureStdout(t, func() { workingTreeCode = runChecks(workingTreeRoots(root), "", false, "links") })
	if workingTreeCode != 1 || !strings.Contains(workingTreeOut, "link target does not resolve: ../../esc-missing/README.md") {
		t.Fatalf("working-tree links = %d, output %q, want 1 and an unresolved-link finding", workingTreeCode, workingTreeOut)
	}
	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (auth.go has an unstaged edit)")
	}
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "links") })
	if got != 1 || !strings.Contains(out, "link target does not resolve: ../../esc-missing/README.md") {
		t.Fatalf("staged links = %d, output %q, want 1 and an unresolved-link finding", got, out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | gitignored doc with an unmarked rule, with nothing else unstaged or untracked (CINCH-006)
func TestStagedChecksIgnoreGitignoredDoc(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, root, ".gitignore", ".docs/local.md\n")
	git("add", ".gitignore")
	git("commit", "-q", "-m", "ignore a local doc")
	writeTestFile(t, filepath.Join(root, ".docs"), "local.md", "1. **LOC-001** unmarked\n")

	if got := runChecks(workingTreeRoots(root), "", false, "rules"); got != 1 {
		t.Fatalf("working-tree rules = %d, want 1 (the ignored doc's LOC-001 is unmarked)", got)
	}
	for name, roots := range map[string]checkRoots{"fast": fastPathRoots(t, root), "slow": slowPathRoots(t, root)} {
		var got int
		out := captureStdout(t, func() { got = runChecks(roots, "", false, "rules") })
		if got != 0 {
			t.Fatalf("%s-path rules = %d, want 0 (an ignored doc isn't part of the commit)\n%s", name, got, out)
		}
	}
}

// fastPathRoots returns the roots stagedSnapshot hands the checks and fails
// unless they read the working tree in place.
func fastPathRoots(t *testing.T, root string) checkRoots {
	t.Helper()
	roots, cleanup, err := stagedSnapshot(root)
	t.Cleanup(cleanup)
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot != root {
		t.Fatalf("fsRoot = %q, want repoRoot %q (untracked and ignored files must not force a snapshot)", roots.fsRoot, root)
	}
	if roots.index == nil {
		t.Fatal("index = nil, want the indexed paths the checks may read")
	}
	return roots
}

// slowPathRoots is the checkout-index snapshot, whatever stagedSnapshot would
// have chosen.
func slowPathRoots(t *testing.T, root string) checkRoots {
	t.Helper()
	index, err := readIndex(root)
	if err != nil {
		t.Fatalf("readIndex: %v", err)
	}
	roots, cleanup, err := checkoutSnapshot(root, index)
	t.Cleanup(cleanup)
	if err != nil {
		t.Fatalf("checkoutSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot directory")
	}
	return roots
}

// renderedRepo commits a cinch.yml and everything cinch render writes for it.
func renderedRepo(t *testing.T, root string) func(args ...string) {
	t.Helper()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: .docs\n")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	return git
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | file in the working tree that the commit does not contain (CINCH-006)
func TestStagedUntrackedFileKeepsFastPath(t *testing.T) {
	root := t.TempDir()
	stagedTestRepo(t, root)
	writeTestFile(t, root, "untracked.txt", "not in the commit\n")

	fastPathRoots(t, root)
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | untracked doc with an unmarked rule, on the fast path (CINCH-006)
func TestStagedFastPathIgnoresUntrackedDoc(t *testing.T) {
	root := t.TempDir()
	stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, ".docs"), "scratch.md", "# Scratch\n\n1. **SCR-001** unmarked\n")

	if got := runChecks(workingTreeRoots(root), "", false, "rules"); got != 1 {
		t.Fatalf("working-tree rules = %d, want 1 (the untracked doc's SCR-001 is unmarked)", got)
	}
	roots := fastPathRoots(t, root)
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "rules") })
	if got != 0 {
		t.Fatalf("staged rules = %d, want 0 (an untracked doc isn't part of the commit)\n%s", got, out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: links | caught | doc link to a file or directory that is only untracked, on the fast path (CINCH-006)
func TestStagedFastPathCatchesLinkToUntrackedFile(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	// ../internal is a directory holding an indexed file, so it resolves.
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md",
		stagedOneRuleDoc+"\n[dir](../internal)\n[file](../internal/new.go)\n[scratch](../scratch)\n")
	git("add", ".docs/rules.md")
	git("commit", "-q", "-m", "link to files the commit lacks")
	writeTestFile(t, filepath.Join(root, "internal"), "new.go", "package auth\n")
	if err := os.MkdirAll(filepath.Join(root, "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "scratch"), "a.txt", "untracked\n")

	roots := fastPathRoots(t, root)
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "links") })
	if got != 1 {
		t.Fatalf("staged links = %d, want 1\n%s", got, out)
	}
	for _, target := range []string{"../internal/new.go", "../scratch"} {
		if !strings.Contains(out, "link target does not resolve: "+target+"\n") {
			t.Fatalf("output = %q, want %q reported unresolved", out, target)
		}
	}
	if n := strings.Count(out, "link target does not resolve"); n != 2 {
		t.Fatalf("output = %q, want exactly 2 unresolved links (../internal is an indexed directory)", out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: generated | not a defect | untracked file with cinch's generated header in a rendered directory, on the fast path (CINCH-006)
func TestStagedFastPathIgnoresUntrackedGeneratedLookalike(t *testing.T) {
	root := t.TempDir()
	renderedRepo(t, root)
	workflows := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	writeTestFile(t, workflows, "stray.md", readTestFile(t, workflows, "docs-philosophy.md"))

	if got := runChecks(workingTreeRoots(root), "", false, "generated"); got != 1 {
		t.Fatalf("working-tree generated = %d, want 1 (stray.md carries the header and isn't rendered)", got)
	}
	roots := fastPathRoots(t, root)
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "generated") })
	if got != 0 {
		t.Fatalf("staged generated = %d, want 0 (an untracked file isn't part of the commit)\n%s", got, out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: generated | caught | commit that leaves out a rendered file, on the fast path (CINCH-006)
func TestStagedFastPathCatchesRenderedFileLeftOutOfCommit(t *testing.T) {
	root := t.TempDir()
	git := renderedRepo(t, root)
	rendered := filepath.Join(defaultDocsPath, workflowsSubdir, "docs-philosophy.md")
	// The file stays on disk, so the working tree still looks fully rendered.
	git("rm", "--cached", "-q", rendered)

	if got := runChecks(workingTreeRoots(root), "", false, "generated"); got != 0 {
		t.Fatalf("working-tree generated = %d, want 0 (the file is on disk)", got)
	}
	roots := fastPathRoots(t, root)
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "generated") })
	if got != 1 || !strings.Contains(out, rendered+":1: missing") {
		t.Fatalf("staged generated = %d, output %q, want 1 and %s reported missing", got, out, rendered)
	}
}

// cinch:rule CINCH-006
// catch-matrix: generated | not a defect | rendered output present only in the working tree, on the fast path (CINCH-006)
func TestStagedFastPathTreatsUntrackedRenderAsNotRendered(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: .docs\n")
	git("add", "cinch.yml")
	git("commit", "-q", "-m", "manifest only")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}

	roots := fastPathRoots(t, root)
	m, err := loadManifestOptional(roots.fsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result := checkGenerated(roots, m, nil); result.noOp == "" || len(result.findings) != 0 {
		t.Fatalf("checkGenerated = %+v, want the not-yet-rendered no-op (nothing rendered is in the commit)", result)
	}
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | untracked cinch.yml that would redirect the docs root (CINCH-006)
func TestStagedUntrackedManifestForcesSnapshot(t *testing.T) {
	root := t.TempDir()
	stagedTestRepo(t, root)
	writeTestFile(t, root, "cinch.yml", "paths:\n  docs: elsewhere\n")

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		t.Fatalf("stagedSnapshot: %v", err)
	}
	if roots.fsRoot == root {
		t.Fatal("fsRoot = repoRoot, want a snapshot (cinch.yml is read by path and isn't in the commit)")
	}
	var got int
	out := captureStdout(t, func() { got = runChecks(roots, "", false, "rules") })
	if got != 0 || strings.Contains(out, "failed to scan") {
		t.Fatalf("staged rules = %d, output %q, want 0 reading the committed .docs", got, out)
	}
}

// cinch:rule CINCH-006
// catch-matrix: links | not a defect | untracked and ignored docs and link targets, fast path against snapshot (CINCH-006)
func TestStagedFastAndSlowPathsPrintTheSame(t *testing.T) {
	root := t.TempDir()
	git := renderedRepo(t, root)
	writeTestFile(t, root, ".gitignore", ".docs/local.md\nignored.txt\n")
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedTwoRulesDoc+
		"\n[broken](missing.md)\n[untracked](scratch.md)\n[ignored](local.md)\n[ignored file](../ignored.txt)\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode)
	git("add", "-A")
	git("commit", "-q", "-m", "docs with links")

	writeTestFile(t, filepath.Join(root, ".docs"), "scratch.md", "# Scratch\n\n1. **SCR-001** unmarked\n")
	writeTestFile(t, filepath.Join(root, ".docs"), "local.md", "# Local\n\n1. **LOC-001** unmarked\n")
	writeTestFile(t, root, "ignored.txt", "ignored\n")
	writeTestFile(t, root, "untracked.txt", "untracked\n")
	workflows := filepath.Join(root, defaultDocsPath, workflowsSubdir)
	writeTestFile(t, workflows, "stray.md", readTestFile(t, workflows, "docs-philosophy.md"))

	fast, slow := fastPathRoots(t, root), slowPathRoots(t, root)
	var fastCode, slowCode int
	fastOut := captureStdout(t, func() { fastCode = preCommitChecks(fast) })
	slowOut := captureStdout(t, func() { slowCode = preCommitChecks(slow) })
	if fastCode != 1 || slowCode != 1 {
		t.Fatalf("exit codes: fast %d, slow %d, want both 1 (the committed docs have broken links and an unmarked rule)\nfast:\n%s\nslow:\n%s", fastCode, slowCode, fastOut, slowOut)
	}
	if fastOut != slowOut {
		t.Fatalf("fast and slow paths differ:\nfast:\n%s\nslow:\n%s", fastOut, slowOut)
	}
	for _, leaked := range []string{"SCR-001", "LOC-001", "stray.md"} {
		if strings.Contains(fastOut, leaked) {
			t.Fatalf("output names %s, which the commit doesn't contain:\n%s", leaked, fastOut)
		}
	}
}

// cinch:rule CINCH-006
// catch-matrix: rules | not a defect | docs directory that exists only in the working tree, fast path against snapshot (CINCH-006)
func TestStagedFastAndSlowPathsAgreeOnUntrackedDocsDirectory(t *testing.T) {
	root := t.TempDir()
	git := initTestGitRepo(t, root)
	writeTestFile(t, root, "a.txt", "tracked\n")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	if err := os.MkdirAll(filepath.Join(root, ".docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", stagedOneRuleDoc)

	fast, slow := fastPathRoots(t, root), slowPathRoots(t, root)
	var fastCode, slowCode int
	fastOut := captureStdout(t, func() { fastCode = runChecks(fast, "", false, "links", "rules", "index") })
	slowOut := captureStdout(t, func() { slowCode = runChecks(slow, "", false, "links", "rules", "index") })
	if fastCode != slowCode || fastOut != slowOut {
		t.Fatalf("fast path (exit %d) and snapshot (exit %d) differ:\nfast:\n%s\nslow:\n%s", fastCode, slowCode, fastOut, slowOut)
	}
	if !strings.Contains(fastOut, "failed to scan") {
		t.Fatalf("output = %q, want the docs directory reported as missing from the commit", fastOut)
	}
}

// cinch:rule CINCH-006
func TestCheckRootsInCommit(t *testing.T) {
	index := newIndexSet([]string{"cinch.yml", ".docs/rules.md", "internal/auth/auth.go"})
	cases := []struct {
		name   string
		fsRoot string
		path   string
		want   bool
	}{
		{"indexed file", "/r", "/r/.docs/rules.md", true},
		{"ancestor directory", "/r", "/r/internal/auth", true},
		{"top-level ancestor", "/r", "/r/internal", true},
		{"untracked file", "/r", "/r/internal/new.go", false},
		{"untracked directory", "/r", "/r/scratch", false},
		{"directory prefix is not an ancestor", "/r", "/r/inter", false},
		{"fsRoot itself", "/r", "/r", true},
		{"outside fsRoot", "/r", "/sibling/README.md", true},
		{"parent of fsRoot", "/r/repo", "/r", true},
		{"dot fsRoot, indexed", ".", ".docs/rules.md", true},
		{"dot fsRoot, untracked", ".", "scratch/a.txt", false},
		{"dot fsRoot, itself", ".", ".", true},
		{"dot fsRoot, outside", ".", "../sibling/README.md", true},
		{"relative fsRoot, indexed", "tmp/repo", "tmp/repo/internal/auth/auth.go", true},
		{"relative fsRoot, untracked", "tmp/repo", "tmp/repo/internal/auth/new.go", false},
		{"dotdot fsRoot, indexed", "../repo", "../repo/cinch.yml", true},
		{"dotdot fsRoot, untracked", "../repo", "../repo/other.yml", false},
		{"unclean path", "/r", "/r/internal/../scratch", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			roots := checkRoots{repoRoot: c.fsRoot, fsRoot: c.fsRoot, index: index}
			if got := roots.inCommit(c.path); got != c.want {
				t.Fatalf("inCommit(%q) with fsRoot %q = %v, want %v", c.path, c.fsRoot, got, c.want)
			}
		})
	}
	t.Run("working tree", func(t *testing.T) {
		if !workingTreeRoots("/r").inCommit("/r/anything") {
			t.Fatal("inCommit = false, want true when there is no index")
		}
	})
}
