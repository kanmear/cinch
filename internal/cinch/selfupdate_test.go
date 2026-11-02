package cinch

import (
	"testing"
)

// withReleaseCheckDisabled prevents maybeInstallLatest from making a live
// network call during a test, restoring the prior value afterward.
func withReleaseCheckDisabled(t *testing.T) {
	t.Helper()
	old := releaseCheckDisabled
	releaseCheckDisabled = true
	t.Cleanup(func() { releaseCheckDisabled = old })
}

func TestIsNewerRelease(t *testing.T) {
	cases := []struct {
		name              string
		installed, remote string
		want              bool
	}{
		{"older installed", "1.2.3", "1.2.4", true},
		{"same version", "1.2.3", "1.2.3", false},
		{"newer installed", "1.2.4", "1.2.3", false},
		{"major bump", "1.2.3", "2.0.0", true},
		{"build metadata on installed", "1.2.3+c", "1.2.4", true},
		{"build metadata on remote", "1.2.3", "1.2.3+c", false},
		{"dev build", "dev", "1.2.3", false},
		{"unparseable remote", "1.2.3", "not-a-version", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isNewerRelease(c.installed, c.remote); got != c.want {
				t.Errorf("isNewerRelease(%q, %q) = %v, want %v", c.installed, c.remote, got, c.want)
			}
		})
	}
}
