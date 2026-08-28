package semver

import (
	"strconv"
	"strings"
)

type Version struct{ major, minor, patch int }

func Parse(s string) (v Version, ok bool) {
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	parts := strings.SplitN(s, ".", 3)
	nums := [3]int{}
	for i := range nums {
		if i >= len(parts) {
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return Version{}, false
		}
		nums[i] = n
	}
	return Version{nums[0], nums[1], nums[2]}, true
}

func Equal(a, b string) bool {
	pa, oka := Parse(a)
	pb, okb := Parse(b)
	if !oka || !okb {
		return a == b
	}
	return pa == pb
}

func Less(a, b Version) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// AtLeast reports whether version satisfies a >= minimum constraint.
// Build metadata ("1.2.3+a") is ignored; anything else that doesn't parse
// (e.g. "dev", or pre-releases like "1.2.3-dev") falls back to string
// equality, matching Equal's behavior.
func AtLeast(version, minimum string) bool {
	pv, okv := Parse(version)
	pm, okm := Parse(minimum)
	if !okv || !okm {
		return version == minimum
	}
	return !Less(pv, pm)
}
