package cinch

import "fmt"

var Version = "dev"

const requireCinchKey = "require.cinch"

func checkPin(root, version string) checkResult {
	want, ok, _ := manifestSetting(root, requireCinchKey)
	if !ok {
		return checkResult{noOp: "require.cinch is not set in cinch.yml — opt-in, not configured"}
	}
	if version == "dev" {
		return checkResult{noOp: "binary is an unreleased (dev) build — require.cinch is not checked"}
	}
	if semverEqual(version, want) {
		return checkResult{}
	}
	return checkResult{findings: []finding{{
		check: "core", level: "error", file: manifestPath, line: 1,
		message: fmt.Sprintf("installed cinch %s does not match require.cinch %s — reinstall and re-run cinch render", version, want),
	}}}
}
