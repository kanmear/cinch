package cinch

import (
	"os"
	"path/filepath"
	"reflect"
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

// TestCmdHookPreCommitImpactNeverBlocks guards the core impact design
// constraint: an advisory with something to say must never flip a
// pre-commit hook that would otherwise pass into a failing one.
func TestCmdHookPreCommitImpactNeverBlocks(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")

	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "a.md", "# Title\n\n1. **RUL-1** some rule\n")
	src := filepath.Join(root, "internal", "pkg")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, "impl.go", "package pkg\n\n// cinch:rule RUL-1\nfunc F() {}\n")

	git("add", "-A")

	if got := preCommitChecks(workingTreeRoots(root)); got != 0 {
		t.Fatalf("preCommitChecks = %d, want 0 (clean corpus)", got)
	}
	hits, err := buildImpact(root, filepath.Join(root, ".docs"), []string{"internal/pkg/impl.go"}, nil)
	if err != nil || len(hits) == 0 {
		t.Fatalf("buildImpact = %v, %v, want a nonempty marker hit to make this test meaningful", hits, err)
	}

	if got := cmdHookPreCommit(root); got != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0 — a nonempty impact advisory must not block the commit", got)
	}
}

// TestCmdHookPreCommitImpactErrorSwallowed guards the stricter half of the
// same constraint: even a buildImpact scan error (malformed owns:
// frontmatter, here — checks never look at frontmatter, so this doesn't
// touch them) must not surface as a failure inside the hook path.
func TestCmdHookPreCommitImpactErrorSwallowed(t *testing.T) {
	root := t.TempDir()
	git := gitTestHelper(t, root)
	git("init", "-q")
	git("config", "user.email", "cinch@test")
	git("config", "user.name", "cinch test")

	docs := filepath.Join(root, ".docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, docs, "malformed.md", "---\nowns: \"not-a-list\"\n---\n# Malformed\n\nno rules here\n")
	git("add", "-A")

	if got := preCommitChecks(workingTreeRoots(root)); got != 0 {
		t.Fatalf("preCommitChecks = %d, want 0 (malformed frontmatter is invisible to the checks)", got)
	}
	if _, err := buildImpact(root, filepath.Join(root, ".docs"), nil, nil); err == nil {
		t.Fatal("buildImpact = nil error, want an error to make this test meaningful")
	}

	if got := cmdHookPreCommit(root); got != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0 — a buildImpact scan error must be swallowed, not surfaced", got)
	}
}

// Without -z, git C-quotes a path holding non-ASCII bytes or a '"', so
// internal/ü.go would come back as "internal/\303\274.go", quotes included,
// and no longer start with a when: prefix like internal/.
func TestPathListingsReturnVerbatimNames(t *testing.T) {
	names := []string{`internal/say "hi".go`, "internal/ü.go"}
	writeAll := func(t *testing.T, root, body string) {
		t.Helper()
		for _, name := range names {
			writeTestFile(t, root, name, body)
		}
	}
	cases := []struct {
		name string
		list func(t *testing.T, root string, git func(args ...string)) ([]string, error)
	}{
		{"staged", func(t *testing.T, root string, git func(args ...string)) ([]string, error) {
			writeAll(t, root, "staged\n")
			git("add", "-A")
			return stagedPaths(root)
		}},
		{"unstaged", func(t *testing.T, root string, git func(args ...string)) ([]string, error) {
			writeAll(t, root, "committed\n")
			git("add", "-A")
			git("commit", "-q", "-m", "add")
			writeAll(t, root, "unstaged\n")
			return unstagedPaths(root)
		}},
		{"committed", func(t *testing.T, root string, git func(args ...string)) ([]string, error) {
			writeAll(t, root, "committed\n")
			git("add", "-A")
			git("commit", "-q", "-m", "add")
			return committedFiles(root)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			git := initTestGitRepo(t, root)
			writeTestFile(t, root, "README.md", "seed\n")
			if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
				t.Fatal(err)
			}
			git("add", "-A")
			git("commit", "-q", "-m", "seed")

			got, err := tc.list(t, root, git)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, names) {
				t.Fatalf("paths = %q, want %q", got, names)
			}
			if !hookWhenMatches(got, []string{"internal/"}) {
				t.Fatalf("hookWhenMatches(%q, [internal/]) = false, want true", got)
			}
		})
	}
}
