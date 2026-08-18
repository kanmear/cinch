package cinch

import "testing"

func TestHookWhenMatches(t *testing.T) {
	cases := []struct {
		name         string
		staged, when []string
		want         bool
	}{
		{"nil staged", nil, []string{"src"}, true},
		{"empty when", []string{"src/a.go"}, nil, true},
		{"prefix match", []string{"src/a.go", "b.txt"}, []string{"src"}, true},
		{"second prefix match", []string{"b.txt"}, []string{"src", "b"}, true},
		{"no match", []string{"b.txt"}, []string{"src"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hookWhenMatches(tc.staged, tc.when); got != tc.want {
				t.Fatalf("hookWhenMatches = %v, want %v", got, tc.want)
			}
		})
	}
}
