package cinch

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is the cinch release the running binary was built from. main()
// sets this from its own build-stamped Version before dispatching any
// command, so every call path — including hook.go's internal CmdCheck
// invocations, which have no other route to the binary's version — sees the
// same value checkPin compares against require.cinch.
var Version = "devel"

const requireCinchKey = "require.cinch"

// pinResult separates a real mismatch finding from the no-op case: no
// require.cinch key, or an unreleased (devel) binary, is not a failure —
// the pin is opt-in, and a local dev build must never redden a consumer
// that pins a release.
type pinResult struct {
	Findings []Finding
	NoOp     string
}

// checkPin compares version (the running binary's) against the manifest's
// require.cinch key, when both are present. No manifest, or no
// require.cinch key, is not an error — the pin is opt-in, and absence
// changes nothing. An unreleased (devel) binary skips the comparison
// outright: a local dev build must never redden a consumer that pins a
// release.
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
		Message: fmt.Sprintf("installed cinch %s does not match require.cinch %s — run `make install` (or your build's equivalent) to update the binary, then `cinch render` to re-sync generated docs", version, want),
	}}}
}

// semver is a parsed major.minor.patch version, compared by value equality.
type semver struct{ major, minor, patch int }

// parseSemver parses an "x.y.z" string, treating a missing trailing
// component as 0 ("1.2" == "1.2.0"). ok is false when a present component
// isn't a plain integer.
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

// semverEqual compares a and b by semver precedence when both parse
// cleanly, falling back to plain string equality otherwise — a malformed
// require.cinch value (or an installed version that isn't "x.y.z") still
// gets a decidable answer instead of a silent false match.
func semverEqual(a, b string) bool {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if !oka || !okb {
		return a == b
	}
	return pa == pb
}
