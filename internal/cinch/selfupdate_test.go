package cinch

import "testing"

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
		installed, remote string
		want              bool
	}{
		{"1.2.3", "1.2.4", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", "2.0.0", true},
		{"1.2.3+c", "1.2.4", true},
		{"1.2.3", "1.2.3+c", false},
		{"dev", "1.2.3", false},
		{"1.2.3", "not-a-version", false},
	}
	for _, c := range cases {
		if got := isNewerRelease(c.installed, c.remote); got != c.want {
			t.Errorf("isNewerRelease(%q, %q) = %v, want %v", c.installed, c.remote, got, c.want)
		}
	}
}
