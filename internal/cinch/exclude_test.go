package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// excludeMarker is split so this file's own fixtures never read as markers
// when cinch scans its own repository.
const excludeMarker = "// cinch:" + "rule "

// excludeTestRepo commits one rule doc (AUTH-001), the code marking it, a
// cinch.yml with the given paths.exclude entries, and a stray marker naming
// an unknown rule in each of strays — so every stray that isn't excluded
// shows up as a "does not resolve" finding.
func excludeTestRepo(t *testing.T, root string, exclude []string, strays []string) (*manifest, func(args ...string)) {
	t.Helper()
	git := initTestGitRepo(t, root)
	for _, directory := range []string{".docs", "internal", "fixtures"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, root, "cinch.yml", "paths:\n  exclude: ["+strings.Join(exclude, ", ")+"]\n")
	writeTestFile(t, filepath.Join(root, ".docs"), "rules.md", "# Auth rules\n\n1. **AUTH-001** sessions expire\n")
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", "package auth\n\n"+excludeMarker+"AUTH-001\n")
	for i, stray := range strays {
		directory, name := filepath.Split(filepath.Join(root, filepath.FromSlash(stray)))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, directory, name, "package x\n\n"+excludeMarker+"STRAY-"+string(rune('1'+i))+"\n")
	}
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	return m, git
}

// unresolvedFiles returns the file of every "does not resolve" finding.
func unresolvedFiles(report rulesReport) []string {
	var files []string
	for _, f := range report.findings {
		if strings.Contains(f.message, "does not resolve") {
			files = append(files, filepath.ToSlash(f.file))
		}
	}
	return files
}

func TestPathsExcludeMarkerScan(t *testing.T) {
	cases := []struct {
		name    string
		exclude []string
		strays  []string
		want    []string // stray files still reported
	}{
		{"directory prefix", []string{"fixtures/"},
			[]string{"fixtures/a.go", "fixtures/deep/b.go", "fixturesx/c.go"}, []string{"fixturesx/c.go"}},
		{"star stays in one directory", []string{"internal/*_test.go"},
			[]string{"internal/x_test.go", "internal/sub/y_test.go"}, []string{"internal/sub/y_test.go"}},
		{"exact path", []string{"internal/fixture.go"},
			[]string{"internal/fixture.go", "internal/fixture.go.bak"}, []string{"internal/fixture.go.bak"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			m, _ := excludeTestRepo(t, root, tc.exclude, tc.strays)

			report := checkRules(workingTreeRoots(root), filepath.Join(root, ".docs"), markerScanOptionsFor(m))

			got := unresolvedFiles(report)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("unresolved markers in %v, want %v (findings %+v)", got, tc.want, report.findings)
			}
			// AUTH-001's marker sits in a file no entry excludes, so it still binds.
			for _, f := range report.findings {
				if strings.Contains(f.message, "AUTH-001") {
					t.Fatalf("AUTH-001 finding %+v, want its marker in internal/auth.go to count", f)
				}
			}
		})
	}
}

func TestPathsExcludeValidation(t *testing.T) {
	for _, entry := range []string{"[", "/abs", "../outside/"} {
		t.Run(entry, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "cinch.yml", "paths:\n  exclude: ['"+entry+"']\n")

			_, err := loadManifest(root)
			if err == nil || !strings.Contains(err.Error(), entry) {
				t.Fatalf("loadManifest = %v, want an error naming %q", err, entry)
			}
		})
	}
}

func TestPathsExcludeAppliesToRulesJSONAndImpact(t *testing.T) {
	root := t.TempDir()
	m, _ := excludeTestRepo(t, root, []string{"fixtures/"}, nil)
	writeTestFile(t, filepath.Join(root, "fixtures"), "f.go", "package f\n\n"+excludeMarker+"AUTH-001\n")
	docsRoot := filepath.Join(root, ".docs")

	inv, err := buildRulesInventory(root, docsRoot, markerScanOptionsFor(m))
	if err != nil {
		t.Fatalf("buildRulesInventory: %v", err)
	}
	if len(inv.Rules) != 1 || len(inv.Rules[0].Markers) != 1 || inv.Rules[0].Markers[0].File != "internal/auth.go" {
		t.Fatalf("rules = %+v, want AUTH-001 marked only in internal/auth.go", inv.Rules)
	}

	hits, err := buildImpact(root, docsRoot, []string{"fixtures/f.go"}, m.list(pathsExcludeKey))
	if err != nil {
		t.Fatalf("buildImpact: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want none (fixtures/f.go is excluded)", hits)
	}
}

func TestPathsExcludeAppliesToStagedChecks(t *testing.T) {
	root := t.TempDir()
	_, git := excludeTestRepo(t, root, []string{"fixtures/"}, nil)
	writeTestFile(t, filepath.Join(root, "fixtures"), "f.go", "package f\n\n"+excludeMarker+"STRAY-1\n")
	git("add", "fixtures/f.go")
	writeTestFile(t, root, "untracked.txt", "forces a snapshot\n")

	var got int
	out := captureStdout(t, func() { got = stagedPreCommitChecks(root) })
	if got != 0 {
		t.Fatalf("stagedPreCommitChecks = %d, want 0 (the staged stray marker is excluded)\n%s", got, out)
	}
}
