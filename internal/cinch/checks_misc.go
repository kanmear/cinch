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

func checkHooks(root string, m *manifest, mErr error) checkResult {
	if !gitutil.IsRepo(root) {
		return checkResult{noOp: "not a git repository"}
	}

	if mErr != nil || m == nil {
		return checkResult{noOp: "cinch.yml not found — hooks activation not checked"}
	}

	want := hooksPathValue(m)

	out, err := gitutil.Output(root, "config", "--get", "core.hooksPath")
	got := strings.TrimSpace(string(out))
	if err != nil || got == "" {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is not set — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", want)}
	}
	if !sameHooksDirectory(root, got, want) {
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

func checkCommit(messageFile string, m *manifest) checkResult {
	if messageFile == "" {
		return checkResult{noOp: "no commit message file given"}
	}
	pattern, ok := manifestSetting(m, "commit.pattern")
	if !ok {
		return checkResult{noOp: "commit.pattern is not set in cinch.yml — opt-in, not configured"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return checkResult{findings: []finding{{
			check: "commit", level: "error", file: messageFile, line: 1,
			message: "commit.pattern is not a valid regexp: " + err.Error(),
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
	return checkResult{findings: []finding{{
		check: "commit", level: "error", file: messageFile, line: 1,
		message: "commit message does not match commit.pattern (" + pattern + "): " + subject,
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

// checkGenerated reads rendered output under roots.fsRoot but asks git for
// HEAD's cinch.yml in roots.repoRoot: the snapshot directory isn't a
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
		if _, err := os.Stat(filepath.Join(roots.fsRoot, f.destination)); err == nil {
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
		got, err := os.ReadFile(destination)
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
			if expected[path] {
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
