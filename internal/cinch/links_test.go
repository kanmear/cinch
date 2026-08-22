package cinch

import "testing"

func TestLinkTargetIsExempt(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"", true},
		{"https://example.com", true},
		{"http://example.com/x", true},
		{"x://odd", true},
		{"#anchor", true},
		{"mailto:a@b.c", true},
		{"doc.md", false},
		{"../parent.md", false},
		{"./rel/path.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			if got := linkTargetIsExempt(tc.target); got != tc.want {
				t.Fatalf("linkTargetIsExempt(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}
