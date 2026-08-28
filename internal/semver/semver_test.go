package semver

import "testing"

func TestEqual(t *testing.T) {
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
			if got := Equal(tc.a, tc.b); got != tc.want {
				t.Fatalf("Equal(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestAtLeast(t *testing.T) {
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
			if got := AtLeast(tc.version, tc.minimum); got != tc.want {
				t.Fatalf("AtLeast(%q, %q) = %v, want %v", tc.version, tc.minimum, got, tc.want)
			}
		})
	}
}
