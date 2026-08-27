package cinch

import "testing"

func TestSemverEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"1.2.3", "2.0.0", false},
		{"1.2", "1.2.0", true},
		{"1", "1.0.0", true},
		{"1.2.3.4", "1.2.3.4", true},
		{"1.2.3-dev", "1.2.3-dev", true},
		{"1.2.3-dev", "1.2.3", false},
		{"1.2.3+a", "1.2.3", true},
		{"1.2.3+a", "1.2.3+b", true},
		{"1.2.3+a", "1.2.4+b", false},
		{"dev", "dev", true},
		{"dev", "1.0.0", false},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			if got := semverEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("semverEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSemverAtLeast(t *testing.T) {
	cases := []struct {
		version, minimum string
		want             bool
	}{
		{"1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"2.0.0", "1.9.9", true},
		{"1.9.9", "2.0.0", false},
		{"1.3.0", "1.2.9", true},
		{"1.2.0", "1.3.0", false},
		{"1.2.3.4", "1.2.3.4", true},
		{"1.2.3.4", "1.2.3", false},
		{"1.2.3-dev", "1.2.3", false},
		{"0.2.4+a", "0.2.4", true},
		{"0.2.3+a", "0.2.4", false},
		{"1.2.3+hot", "1.2.3", true},
		{"dev", "1.0.0", false},
		{"dev", "dev", true},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.version+" >= "+tc.minimum, func(t *testing.T) {
			if got := semverAtLeast(tc.version, tc.minimum); got != tc.want {
				t.Fatalf("semverAtLeast(%q, %q) = %v, want %v", tc.version, tc.minimum, got, tc.want)
			}
		})
	}
}
