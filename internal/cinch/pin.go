package cinch

import (
	"fmt"
	"strconv"
	"strings"
)

var Version = "devel"

const requireCinchKey = "require.cinch"

type pinResult struct {
	Findings []Finding
	NoOp     string
}

func checkPin(root, version string) pinResult {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return pinResult{NoOp: "require.cinch is not set in cinch.yml — opt-in, not configured"}
	}
	want, ok := m.Vars[requireCinchKey]
	if !ok || want == "" {
		return pinResult{NoOp: "require.cinch is not set in cinch.yml — opt-in, not configured"}
	}
	if version == "devel" {
		return pinResult{NoOp: "binary is an unreleased (devel) build — require.cinch is not checked"}
	}
	if semverEqual(version, want) {
		return pinResult{}
	}
	return pinResult{Findings: []Finding{{
		Check: "core", Level: "error", File: manifestPath, Line: 1,
		Message: fmt.Sprintf("installed cinch %s does not match require.cinch %s — reinstall and re-run cinch render", version, want),
	}}}
}

type semver struct{ major, minor, patch int }

func parseSemver(s string) (v semver, ok bool) {
	parts := strings.SplitN(s, ".", 3)
	nums := [3]int{}
	for i := range nums {
		if i >= len(parts) {
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return semver{}, false
		}
		nums[i] = n
	}
	return semver{nums[0], nums[1], nums[2]}, true
}

func semverEqual(a, b string) bool {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if !oka || !okb {
		return a == b
	}
	return pa == pb
}
