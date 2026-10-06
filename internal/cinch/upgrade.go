package cinch

import (
	"strings"

	"cinch/internal/output"
	"cinch/internal/semver"
)

func CmdUpgrade(root string) int {
	output.Step("installed cinch version: %s", Version)

	if code, handled := maybeInstallLatest(); handled {
		return code
	}

	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	files, err := renderAll(m)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	wrote, err := writeRenderedFiles(root, files)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	removed, err := removeOrphanedFiles(root, files)
	if err != nil {
		return output.Fail("upgrade", err)
	}

	// A floor rises only when this version's output differs from what was
	// committed: a release that renders the same bytes doesn't need one.
	changed := wrote || removed
	reportPinStatus(m, changed)
	if err := syncRequireCinch(root, changed); err != nil {
		return output.Fail("upgrade", err)
	}

	// The files just rendered aren't staged, so the working tree — not the
	// index — is what has to come out clean here.
	return preCommitChecks(workingTreeRoots(root))
}

// reportPinStatus surfaces the ">=" floor cases syncRequireCinch leaves
// silent: a floor it won't raise, either because it's already satisfied with
// no rendered change or because the installed binary is below it.
// syncRequireCinch reports its own rewrites, and both stay silent when unset,
// any, on a dev build, or for an exact pin that already matches.
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
		return
	}
	output.Step("range pin >=%s is NOT satisfied by installed %s — reinstall a newer cinch binary", minimum, Version)
}
