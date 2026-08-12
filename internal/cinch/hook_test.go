package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func manifestFor(t *testing.T, root, content string) *Manifest {
	t.Helper()
	writeFile(t, filepath.Join(root, "cinch.yml"), content)
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	return m
}

func TestDispatchHooks_WhenPrefixMatchesRuns(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "ran")
	m := manifestFor(t, root,
		"hooks:\n  pre-commit:\n    demo:\n      run: touch "+marker+"\n      when: [frontend/]\n")

	ok := dispatchHooks(root, m, "pre-commit", []string{"frontend/src/app.ts"})

	if !ok {
		t.Fatalf("dispatchHooks: want true, got false")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("want the entry to have run (marker file missing): %v", err)
	}
}

func TestDispatchHooks_WhenPrefixNoMatchSkips(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "ran")
	m := manifestFor(t, root,
		"hooks:\n  pre-commit:\n    demo:\n      run: touch "+marker+"\n      when: [frontend/]\n")

	ok := dispatchHooks(root, m, "pre-commit", []string{"backend/main.go"})

	if !ok {
		t.Fatalf("dispatchHooks: want true (a skip is not a failure), got false")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("want the entry to have been skipped, but it ran")
	}
}

func TestDispatchHooks_EmptyWhenAlwaysRuns(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "ran")
	m := manifestFor(t, root, "hooks:\n  pre-commit:\n    demo:\n      run: touch "+marker+"\n")

	ok := dispatchHooks(root, m, "pre-commit", []string{"anything/at/all.txt"})

	if !ok {
		t.Fatalf("dispatchHooks: want true, got false")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("want the entry to have run (no when means always), got: %v", err)
	}
}

func TestDispatchHooks_CommitMsgIgnoresWhen(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "ran")
	m := manifestFor(t, root,
		"hooks:\n  commit-msg:\n    demo:\n      run: touch "+marker+"\n      when: [frontend/]\n")

	// staged == nil is the commit-msg sentinel: `when` never filters.
	ok := dispatchHooks(root, m, "commit-msg", nil)

	if !ok {
		t.Fatalf("dispatchHooks: want true, got false")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("want the entry to have run regardless of when, got: %v", err)
	}
}

func TestDispatchHooks_ExitCodePropagates(t *testing.T) {
	root := t.TempDir()
	m := manifestFor(t, root, "hooks:\n  pre-commit:\n    demo:\n      run: \"false\"\n")

	ok := dispatchHooks(root, m, "pre-commit", []string{"x.txt"})

	if ok {
		t.Fatalf("dispatchHooks: want false when the command exits nonzero, got true")
	}
}

func TestDispatchHooks_MultiFailureAccumulates(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "second-ran")
	m := manifestFor(t, root,
		"hooks:\n  pre-commit:\n    first:\n      run: \"false\"\n    second:\n      run: touch "+marker+"\n")

	ok := dispatchHooks(root, m, "pre-commit", []string{"x.txt"})

	if ok {
		t.Fatalf("dispatchHooks: want false — the first entry failed")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("want the second entry to have run despite the first's failure (accumulate, not fail-fast): %v", err)
	}
}

func TestDispatchHooks_DeclarationOrder(t *testing.T) {
	root := t.TempDir()
	order := filepath.Join(root, "order")
	m := manifestFor(t, root,
		"hooks:\n  pre-commit:\n    zzz:\n      run: sh -c 'echo zzz >> "+order+"'\n    aaa:\n      run: sh -c 'echo aaa >> "+order+"'\n")

	if ok := dispatchHooks(root, m, "pre-commit", nil); !ok {
		t.Fatalf("dispatchHooks: want true, got false")
	}

	data, err := os.ReadFile(order)
	if err != nil {
		t.Fatalf("read order file: %v", err)
	}
	if string(data) != "zzz\naaa\n" {
		t.Fatalf("want declaration order (zzz then aaa), got:\n%s", data)
	}
}

func TestHookWhenMatches(t *testing.T) {
	tests := []struct {
		name   string
		staged []string
		when   []string
		want   bool
	}{
		{"nil staged always matches (commit-msg)", nil, []string{"frontend/"}, true},
		{"empty when always matches", []string{"backend/x.go"}, nil, true},
		{"prefix match", []string{"frontend/src/x.ts"}, []string{"frontend/"}, true},
		{"no prefix match", []string{"backend/x.go"}, []string{"frontend/"}, false},
		{"one of several staged matches", []string{"backend/x.go", "frontend/y.ts"}, []string{"frontend/"}, true},
		{"one of several prefixes matches", []string{"backend/x.go"}, []string{"frontend/", "backend/"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hookWhenMatches(tt.staged, tt.when); got != tt.want {
				t.Fatalf("hookWhenMatches(%v, %v): want %v, got %v", tt.staged, tt.when, tt.want, got)
			}
		})
	}
}
