package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archiveTestRepo(t *testing.T) (root, docsRoot string) {
	t.Helper()
	return plansTestRepo(t, moveManifest, map[string]string{
		"plans/a.md":         "# A\n\n**Status:** complete — shipped in abc\n\n[b](b.md) and [n](../notes/n.md) and [c](c.md)\n",
		"plans/b.md":         "# B\n\n**Status:** in-progress\n\n[a](a.md)\n",
		"plans/c.md":         "# C\n\n**Status:** cut — superseded\n",
		"plans/fix/f.md":     "# F\n\n**Status:** complete\n",
		"plans/archive/z.md": "# Z\n\n**Status:** complete\n",
		"notes/n.md":         "# N\n[a](../plans/a.md#x) and [c](../plans/c.md)\n",
	})
}

// cinch:rule CINCH-019
func TestArchiveCompletePlan(t *testing.T) {
	root, docsRoot := archiveTestRepo(t)

	reports, err := archivePlans(root, docsRoot, []string{".docs/plans/a.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].target != ".docs/plans/archive/a.md" {
		t.Fatalf("reports = %+v", reports)
	}
	if _, err := os.Stat(filepath.Join(docsRoot, "plans", "a.md")); !os.IsNotExist(err) {
		t.Fatalf("source still exists (stat err = %v)", err)
	}
	wantArchived := "# A\n\n**Status:** complete — shipped in abc\n\n[b](../b.md) and [n](../../notes/n.md) and [c](../c.md)\n"
	if got := readFileString(t, filepath.Join(docsRoot, "plans", "archive", "a.md")); got != wantArchived {
		t.Errorf("archived plan =\n%s\nwant\n%s", got, wantArchived)
	}
	if got := readFileString(t, filepath.Join(docsRoot, "plans", "b.md")); !strings.Contains(got, "[a](archive/a.md)") {
		t.Errorf("inbound link from a live plan not rewritten:\n%s", got)
	}
	if got := readFileString(t, filepath.Join(docsRoot, "notes", "n.md")); !strings.Contains(got, "[a](../plans/archive/a.md#x)") {
		t.Errorf("inbound link from another folder not rewritten:\n%s", got)
	}
	requireLinksClean(t, root, docsRoot)
}

// cinch:rule CINCH-019
func TestArchiveCutPlanAndSequentialMoves(t *testing.T) {
	root, docsRoot := archiveTestRepo(t)

	// a links c and c is archived second: the second move must see the first's rewrites.
	if code := CmdPlanArchive(root, []string{".docs/plans/a.md", ".docs/plans/c.md"}); code != 0 {
		t.Fatalf("CmdPlanArchive exit = %d, want 0", code)
	}
	for _, name := range []string{"a.md", "c.md"} {
		if _, err := os.Stat(filepath.Join(docsRoot, "plans", "archive", name)); err != nil {
			t.Errorf("%s not archived: %v", name, err)
		}
	}
	if got := readFileString(t, filepath.Join(docsRoot, "plans", "archive", "a.md")); !strings.Contains(got, "[c](c.md)") {
		t.Errorf("link between two archived plans not resolved from the archive:\n%s", got)
	}
	requireLinksClean(t, root, docsRoot)
}

// cinch:rule CINCH-019
func TestArchiveRefusalsChangeNothing(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		prepare func(t *testing.T, docsRoot string)
		wantErr string
	}{
		{"in-progress plan", []string{".docs/plans/b.md"}, nil, "in-progress"},
		{"fix plan", []string{".docs/plans/fix/f.md"}, nil, "directly in"},
		{"plan already in archive", []string{".docs/plans/archive/z.md"}, nil, "directly in"},
		{"path outside plans", []string{".docs/notes/n.md"}, nil, "directly in"},
		{"path outside the docs root", []string{"outside.md"}, nil, "directly in"},
		{"not markdown", []string{".docs/plans/x.txt"}, func(t *testing.T, docsRoot string) {
			writeTestFile(t, filepath.Join(docsRoot, "plans"), "x.txt", "**Status:** complete\n")
		}, ".md"},
		{"missing file", []string{".docs/plans/none.md"}, nil, "does not exist"},
		{"no status line", []string{".docs/plans/n.md"}, func(t *testing.T, docsRoot string) {
			writeTestFile(t, filepath.Join(docsRoot, "plans"), "n.md", "# N\n")
		}, "Status"},
		{"existing target", []string{".docs/plans/a.md"}, func(t *testing.T, docsRoot string) {
			writeTestFile(t, filepath.Join(docsRoot, "plans", "archive"), "a.md", "# Old A\n")
		}, "already exists"},
		{"second file refused, first must not move", []string{".docs/plans/a.md", ".docs/plans/b.md"}, nil, "in-progress"},
		{"first file refused", []string{".docs/plans/b.md", ".docs/plans/a.md"}, nil, "in-progress"},
		{"same basename twice", []string{".docs/plans/a.md", ".docs/plans/../plans/a.md"}, nil, "twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, docsRoot := archiveTestRepo(t)
			writeTestFile(t, root, "outside.md", "# Outside\n**Status:** complete\n")
			if c.prepare != nil {
				c.prepare(t, docsRoot)
			}
			before := snapshotTree(t, root)

			_, err := archivePlans(root, docsRoot, c.args)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("archivePlans error = %v, want one containing %q", err, c.wantErr)
			}
			requireSameTree(t, before, snapshotTree(t, root))

			if code := CmdPlanArchive(root, c.args); code != 1 {
				t.Fatalf("CmdPlanArchive exit = %d, want 1", code)
			}
			requireSameTree(t, before, snapshotTree(t, root))
		})
	}
}
