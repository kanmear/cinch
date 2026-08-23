package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

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

// TestStagedPathsIncludesDeletions guards the path-scoped hook gates: a
// deletion-only commit must yield a non-empty staged set, otherwise cinch runs
// every path-scoped hook on changes it has no information about.
func TestStagedPathsIncludesDeletions(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")
	deleted := filepath.Join(".agent", "plans", "harness-remediation.md")
	write(deleted, "delete me\n")
	git("add", deleted)
	git("commit", "-q", "-m", "docs: add plan")

	git("rm", "-q", deleted)

	staged, err := stagedPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 1 || staged[0] != deleted {
		t.Fatalf("stagedPaths after deletion-only staging = %v, want [%s]", staged, deleted)
	}

	if hookWhenMatches(staged, []string{"frontend/"}) {
		t.Fatalf("stagedPaths=%v should not match frontend/ gate", staged)
	}
	if !hookWhenMatches(staged, []string{".agent/"}) {
		t.Fatalf("stagedPaths=%v should match .agent/ gate", staged)
	}
}
