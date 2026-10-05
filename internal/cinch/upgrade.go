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

	if err := writeRenderedFiles(root, files); err != nil {
		return output.Fail("upgrade", err)
	}

	if err := removeOrphanedFiles(root, files); err != nil {
		return output.Fail("upgrade", err)
	}

	reportPinStatus(m)
	if err := syncRequireCinch(root); err != nil {
		return output.Fail("upgrade", err)
	}

	// The files just rendered aren't staged, so the working tree — not the
	// index — is what has to come out clean here.
	return preCommitChecks(workingTreeRoots(root))
}

// reportPinStatus surfaces the one case syncRequireCinch leaves silent: a
// ">=" range pin. syncRequireCinch itself already reports an exact-pin
// rewrite via its own output.Step, and stays silent when unset, on a dev
// build, or already matching — reportPinStatus mirrors that silence for
// those cases and only speaks up for ranges.
func reportPinStatus(m *manifest) {
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
		output.Step("range pin >=%s is satisfied by installed %s; no change", minimum, Version)
		return
	}
	output.Step("range pin >=%s is NOT satisfied by installed %s — reinstall a newer cinch binary", minimum, Version)
}
