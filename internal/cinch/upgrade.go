package cinch

import (
	"strings"

	"cinch/internal/output"
	"cinch/internal/semver"
)

func CmdUpgrade(repoRoot string) int {
	output.Step("installed cinch version: %s", Version)

	if code, handled := maybeInstallLatest(); handled {
		return code
	}

	m, err := loadManifest(repoRoot)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	files, err := renderAll(m)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	wrote, err := writeRenderedFiles(repoRoot, files)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	removed, err := removeOrphanedFiles(repoRoot, files)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	// A floor rises only when this version's output differs from what was
	// committed: a release that renders the same bytes doesn't need one.
	changed := wrote || removed
	reportPinStatus(m, changed)
	if err := syncRequireCinch(repoRoot, changed); err != nil {
		return output.Fail("upgrade", err)
	}

	// The files just rendered aren't staged, so the working tree — not the
	// index — is what has to come out clean here.
	return preCommitChecks(workingTreeRoots(repoRoot))
}

// reportPinStatus confirms a satisfied ">=" floor that syncRequireCinch left
// alone. A floor the installed binary is below is not reported here:
// CmdUpgrade ends with preCommitChecks, whose core finding already names it.
// syncRequireCinch reports its own rewrites, and everything stays silent when
// unset, any, on a dev build, or for an exact pin that already matches.
func reportPinStatus(m *manifest, changed bool) {
	want, ok := manifestSetting(m, requireCinchKey)
	if !ok || Version == "dev" {
		return
	}
	minimum, isRange := strings.CutPrefix(strings.TrimSpace(want), ">=")
	if !isRange {
		return
	}
	minimum = strings.TrimSpace(minimum)
	if semver.AtLeast(Version, minimum) {
		if !changed || semver.Equal(Version, minimum) {
			output.Step("range pin >=%s is satisfied by installed %s; no change", minimum, Version)
		}
	}
}
