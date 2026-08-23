package cinch

import (
	"strconv"
	"strings"
)

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

func semverLess(a, b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// semverAtLeast reports whether version satisfies a >= minimum constraint.
// Falls back to string equality when either side doesn't parse, matching
// semverEqual's behavior for non-numeric versions like "dev".
func semverAtLeast(version, minimum string) bool {
	pv, okv := parseSemver(version)
	pm, okm := parseSemver(minimum)
	if !okv || !okm {
		return version == minimum
	}
	return !semverLess(pv, pm)
}
