package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/mdscan"
	"cinch/internal/semver"
)

func checkHooks(repoRoot string, m *manifest, mErr error) checkResult {
	if !gitutil.IsRepo(repoRoot) {
		return checkResult{noOp: "not a git repository"}
	}

	if mErr != nil || m == nil {
		return checkResult{noOp: "cinch.yml not found — hooks activation not checked"}
	}

	want := hooksPathValue(m)

	out, err := gitutil.Output(repoRoot, "config", "--get", "core.hooksPath")
	got := strings.TrimSpace(string(out))
	if err != nil || got == "" {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is not set — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", want)}
	}
	if !sameHooksDirectory(repoRoot, got, want) {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is %q, expected %q (paths.hooks) — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", got, want, want)}
	}
	return checkResult{}
}

// sameHooksDirectory reports whether two core.hooksPath spellings name the
// same directory. Git resolves a relative core.hooksPath against the worktree
// root, so relative values are joined to repoRoot before comparing; symlinks
// are resolved when both directories exist.
func sameHooksDirectory(repoRoot, got, want string) bool {
	resolve := func(path string) string {
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoRoot, path)
		}
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return filepath.Clean(path)
	}
	gotPath, wantPath := resolve(got), resolve(want)
	if gotPath == wantPath {
		return true
	}
	gotReal, gotErr := filepath.EvalSymlinks(gotPath)
	wantReal, wantErr := filepath.EvalSymlinks(wantPath)
	return gotErr == nil && wantErr == nil && gotReal == wantReal
}

// commitState is what git is in the middle of when the commit-msg hook runs:
// a squash merge (SQUASH_MSG in the git dir) or a real merge (MERGE_HEAD).
// Outside a git repository both are false and the subject is judged as an
// ordinary commit.
type commitState struct {
	squash bool
	merge  bool
	branch string // checked out branch while squashing; "" when detached
}

func readCommitState(repoRoot string) commitState {
	if !gitutil.IsRepo(repoRoot) {
		return commitState{}
	}
	out, err := gitutil.Output(repoRoot, "rev-parse", "--git-dir")
	if err != nil {
		return commitState{}
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(gitDir, name))
		return err == nil
	}
	state := commitState{squash: exists("SQUASH_MSG"), merge: exists("MERGE_HEAD")}
	if state.squash {
		if branch, err := gitutil.Output(repoRoot, "symbolic-ref", "--short", "HEAD"); err == nil {
			state.branch = strings.TrimSpace(string(branch))
		}
	}
	return state
}

// The two commit.merge values that are not a pattern.
const (
	commitMergeExempt = "exempt"
	commitMergeReject = "reject"
)

// commitPatternFor picks the setting a subject is judged by. A squash onto a
// branch with a commit.squash entry uses that entry; a real merge uses
// commit.merge (exempt, reject, or a pattern); everything else, including a
// squash onto an unlisted branch and a manifest without the new keys, uses
// commit.pattern exactly as before those keys existed. mode is
// commitMergeExempt or commitMergeReject when commit.merge has that value
// mid merge, and "" otherwise; pattern is unset then.
func commitPatternFor(m *manifest, state commitState) (pattern, source, where, mode string) {
	if state.squash && state.branch != "" {
		key := "commit.squash." + state.branch
		if p, ok := manifestSetting(m, key); ok {
			return p, key, "squash-merge onto " + state.branch + ": ", ""
		}
	}
	if state.merge {
		if mode, ok := manifestSetting(m, "commit.merge"); ok {
			if mode == commitMergeExempt || mode == commitMergeReject {
				return "", "commit.merge", "", mode
			}
			return mode, "commit.merge", "merge commit: ", ""
		}
	}
	p, _ := manifestSetting(m, "commit.pattern")
	return p, "commit.pattern", "", ""
}

func checkCommit(repoRoot, messageFile string, m *manifest) checkResult {
	if messageFile == "" {
		return checkResult{noOp: "no commit message file given"}
	}
	pattern, source, where, mode := commitPatternFor(m, readCommitState(repoRoot))
	switch mode {
	case commitMergeExempt:
		return checkResult{noOp: "merge commit: commit.merge is exempt"}
	case commitMergeReject:
		return checkResult{findings: []finding{{
			check: "commit", level: "error", file: messageFile, line: 1,
			message: "merge commit: commit.merge is reject — land the branch as a squash commit with make merge (git merge --squash), not a merge commit",
		}}}
	}
	if pattern == "" {
		return checkResult{noOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "commit", level: "error", file: messageFile, line: 1,
			message: source + " is not a valid regexp: " + err.Error(),
		}}}
	}
	data, err := os.ReadFile(messageFile)
	if err != nil {
		return checkResult{noOp: "could not read " + messageFile}
	}
	subject, _, _ := strings.Cut(string(data), "\n")
	if re.MatchString(subject) {
		return checkResult{}
	}
	message := where + "commit message does not match " + source + " (" + pattern + "): " + subject
	// git's default merge subjects ("Merge branch …", "Merge tag …") are easy to
	// produce by accident; say where the subject came from and how to override it.
	if source == "commit.pattern" && strings.HasPrefix(subject, "Merge ") {
		message += " — this is git's default merge message; commit the merge with -m '<a subject matching commit.pattern>' (or set commit.merge: exempt)"
	}
	return checkResult{findings: []finding{{
		check: "commit", level: "error", file: messageFile, line: 1,
		message: message,
	}}}
}

var Version = "dev"

const requireCinchKey = "require.cinch"

// requireCinchAny is the explicit opt-out: the repo deliberately accepts any
// cinch version, and neither checkPin nor syncRequireCinch acts on it.
const requireCinchAny = "any"

func checkPin(version string, m *manifest) checkResult {
	want, ok := manifestSetting(m, requireCinchKey)
	if !ok {
		suggestion := "'>=<version>'"
		if version != "dev" {
			suggestion = "'>=" + version + "'"
		}
		return checkResult{noOp: "require.cinch is not set — add require: {cinch: " + suggestion + "} to guard against older binaries, or set it to any"}
	}
	if strings.TrimSpace(want) == requireCinchAny {
		return checkResult{noOp: "require.cinch is any — version pin disabled"}
	}
	if version == "dev" {
		return checkResult{noOp: "binary is an unreleased (dev) build — require.cinch is not checked"}
	}

	if minimum, isMinimum := strings.CutPrefix(want, ">="); isMinimum {
		minimum = strings.TrimSpace(minimum)
		if semver.AtLeast(version, minimum) {
			return checkResult{}
		}
		return checkResult{findings: []finding{{
			check: "core", level: "error", file: manifestPath, line: 1,
			message: fmt.Sprintf("installed cinch %s does not satisfy require.cinch >=%s — reinstall and re-run cinch render", version, minimum),
		}}}
	}

	if semver.Equal(version, want) {
		return checkResult{}
	}
	return checkResult{findings: []finding{{
		check: "core", level: "error", file: manifestPath, line: 1,
		message: fmt.Sprintf("installed cinch %s does not match require.cinch %s — reinstall and re-run cinch render", version, want),
	}}}
}

// checkGenerated reads rendered output under roots.fsRoot, and only what the
// commit contains: a destination that isn't indexed counts as absent. It asks
// git for HEAD's cinch.yml in roots.repoRoot: the snapshot directory isn't a
// repository.
func checkGenerated(roots checkRoots, m *manifest, mErr error) checkResult {
	if mErr != nil || m == nil {
		return checkResult{noOp: "cinch render has not run — nothing to verify"}
	}

	files, err := renderAll(m)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "generated", level: "error", file: manifestPath, line: 1,
			message: err.Error() + " — cinch render fails here, so no generated output can be verified",
		}}}
	}

	rendered := false
	for _, f := range files {
		destination := filepath.Join(roots.fsRoot, f.destination)
		if _, err := os.Stat(destination); err == nil && roots.inCommit(destination) {
			rendered = true
			break
		}
	}
	if !rendered {
		return checkResult{noOp: "cinch render has not run — nothing to verify"}
	}

	var findings []finding
	for _, f := range files {
		destination := filepath.Join(roots.fsRoot, f.destination)
		want := header(f.source, f.body, f.style) + f.body
		got, err := roots.readFile(destination)
		switch {
		case err != nil:
			findings = append(findings, finding{
				check: "generated", level: "error", file: f.destination, line: 1,
				message: "missing — run 'cinch render' (or revert the edit)",
			})
		case string(got) != want:
			findings = append(findings, finding{
				check: "generated", level: "error", file: f.destination, line: 1,
				message: "does not match a fresh render — run 'cinch render' (or revert the edit)",
			})
		}
	}

	for _, rel := range orphanedGeneratedFiles(roots, files) {
		findings = append(findings, finding{
			check: "generated", level: "error", file: rel, line: 1,
			message: "orphaned generated file, no longer produced by cinch render — run 'cinch render' to remove it",
		})
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].file < findings[j].file })
	return checkResult{findings: findings}
}

// orphanedGeneratedFiles returns fsRoot-relative paths of files that carry
// cinch's generated header but aren't among files. It looks only
// at the top level of the current render directories plus those HEAD's
// cinch.yml implies, so a docs or hooks path change still finds what the
// old path left behind. HEAD comes from repoRoot: a staged snapshot under
// fsRoot isn't a repository.
func orphanedGeneratedFiles(roots checkRoots, files []renderFile) []string {
	expected := map[string]bool{}
	renderDirectories := map[string]bool{}
	for _, f := range files {
		destination := filepath.Join(roots.fsRoot, f.destination)
		expected[destination] = true
		renderDirectories[filepath.Dir(destination)] = true
	}

	if previousData, ok := gitutil.Show(roots.repoRoot, "HEAD:"+manifestPath); ok {
		if previousManifest, err := parseManifestBytes(previousData, manifestPath+"@HEAD"); err == nil {
			renderDirectories[filepath.Join(roots.fsRoot, docsPathValue(previousManifest), workflowsSubdir)] = true
			renderDirectories[filepath.Join(roots.fsRoot, hooksPathValue(previousManifest))] = true
		}
	}

	var orphans []string
	for directory := range renderDirectories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue
			}
			path := filepath.Join(directory, e.Name())
			if expected[path] || !roots.inCommit(path) {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil || !hasGeneratedHeader(string(data)) {
				continue
			}
			orphans = append(orphans, mdscan.RelTo(roots.fsRoot, path))
		}
	}
	sort.Strings(orphans)
	return orphans
}
