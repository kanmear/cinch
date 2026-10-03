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
	if filepath.Clean(got) != filepath.Clean(want) {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is %q, expected %q (paths.hooks) — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", got, want, want)}
	}
	return checkResult{}
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

func checkPin(version string, m *manifest) checkResult {
	want, ok := manifestSetting(m, requireCinchKey)
	if !ok {
		return checkResult{noOp: "require.cinch is not set in cinch.yml — opt-in, not configured"}
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

	expected := map[string]bool{}
	renderDirectories := map[string]bool{}
	var findings []finding
	for _, f := range files {
		destination := filepath.Join(roots.fsRoot, f.destination)
		expected[destination] = true
		renderDirectories[filepath.Dir(destination)] = true

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

	if previousData, ok := gitutil.Show(roots.repoRoot, "HEAD:"+manifestPath); ok {
		if previousManifest, err := parseManifestBytes(previousData, manifestPath+"@HEAD"); err == nil {
			renderDirectories[filepath.Join(roots.fsRoot, docsPathValue(previousManifest), workflowsSubdir)] = true
			renderDirectories[filepath.Join(roots.fsRoot, hooksPathValue(previousManifest))] = true
		}
	}

	for directory := range renderDirectories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
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
			rel := mdscan.RelTo(roots.fsRoot, path)
			findings = append(findings, finding{
				check: "generated", level: "error", file: rel, line: 1,
				message: "orphaned generated file, no longer produced by cinch render — delete it (cinch render never removes files)",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].file < findings[j].file })
	return checkResult{findings: findings}
}
