package cinch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	moveManifest = "paths:\n  docs: .docs\n"
	codeFence    = "```"
)

// snapshotTree records every file's bytes and every directory under root, so
// a refused command can be shown to have changed nothing at all.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			snapshot[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func requireSameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	for path, content := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("%s was removed", path)
		} else if got != content {
			t.Errorf("%s changed:\n%s\nwas:\n%s", path, got, content)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s was created", path)
		}
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireLinksClean(t *testing.T, root, docsRoot string) {
	t.Helper()
	report := checkLinks(workingTreeRoots(root), docsRoot)
	if len(report.findings) != 0 {
		t.Fatalf("checkLinks findings = %+v, want none", report.findings)
	}
}

// moveTestRepo is a docs tree with an inbound link from another folder, a
// sibling, outbound links to a doc, an image and code outside the docs root,
// and link syntax inside inline code and a fenced block.
func moveTestRepo(t *testing.T) (root, docsRoot string) {
	t.Helper()
	root, docsRoot = plansTestRepo(t, moveManifest, map[string]string{
		"guide/a.md": "# A\n" +
			"See [b](b.md#sec \"Title\") and [other](../notes/n.md).\n" +
			"Img [img](../img/d.png), code [code](../../src/x.go), self [me](a.md#top).\n" +
			"Angle [ang](<../notes/n.md>), web [w](https://example.com/a.md).\n" +
			"Inline `[c](b.md)` and `[d](../notes/n.md)` stay.\n" +
			codeFence + "\n[f](b.md)\n" + codeFence + "\n",
		"guide/b.md": "# B\n[a](a.md#top)\n",
		"notes/n.md": "# N\n" +
			"[a](../guide/a.md 'T') and `[a](../guide/a.md)`\n" +
			codeFence + "\n[a](../guide/a.md)\n" + codeFence + "\n",
		"img/d.png": "png",
	})
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "src"), "x.go", "package x\n")
	return root, docsRoot
}

// cinch:rule CINCH-018
// cinch:rule CINCH-005
func TestMoveRewritesLinksBothWays(t *testing.T) {
	root, docsRoot := moveTestRepo(t)

	report, err := moveFile(root, docsRoot, ".docs/guide/a.md", ".docs/ref/deep/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if report.source != ".docs/guide/a.md" || report.target != ".docs/ref/deep/a.md" {
		t.Fatalf("report paths = %q -> %q", report.source, report.target)
	}

	if _, err := os.Stat(filepath.Join(docsRoot, "guide", "a.md")); !os.IsNotExist(err) {
		t.Fatalf("source still exists (stat err = %v)", err)
	}
	wantMoved := "# A\n" +
		"See [b](../../guide/b.md#sec \"Title\") and [other](../../notes/n.md).\n" +
		"Img [img](../../img/d.png), code [code](../../../src/x.go), self [me](a.md#top).\n" +
		"Angle [ang](<../../notes/n.md>), web [w](https://example.com/a.md).\n" +
		"Inline `[c](b.md)` and `[d](../notes/n.md)` stay.\n" +
		codeFence + "\n[f](b.md)\n" + codeFence + "\n"
	if got := readFileString(t, filepath.Join(docsRoot, "ref", "deep", "a.md")); got != wantMoved {
		t.Errorf("moved file =\n%s\nwant\n%s", got, wantMoved)
	}
	if got, want := readFileString(t, filepath.Join(docsRoot, "guide", "b.md")), "# B\n[a](../ref/deep/a.md#top)\n"; got != want {
		t.Errorf("sibling =\n%s\nwant\n%s", got, want)
	}
	wantNotes := "# N\n" +
		"[a](../ref/deep/a.md 'T') and `[a](../guide/a.md)`\n" +
		codeFence + "\n[a](../guide/a.md)\n" + codeFence + "\n"
	if got := readFileString(t, filepath.Join(docsRoot, "notes", "n.md")); got != wantNotes {
		t.Errorf("inbound from another folder =\n%s\nwant\n%s", got, wantNotes)
	}
	requireLinksClean(t, root, docsRoot)

	counts := map[string]int{}
	for _, rewrite := range report.rewrites {
		counts[rewrite.file] = rewrite.links
	}
	want := map[string]int{".docs/ref/deep/a.md": 5, ".docs/guide/b.md": 1, ".docs/notes/n.md": 1}
	for file, links := range want {
		if counts[file] != links {
			t.Errorf("rewrites[%s] = %d, want %d (all: %v)", file, counts[file], links, counts)
		}
	}
	if len(counts) != len(want) {
		t.Errorf("rewrites = %v, want exactly %v", counts, want)
	}
}

// cinch:rule CINCH-018
func TestMoveRenameInPlaceRewritesSelfLink(t *testing.T) {
	root, docsRoot := moveTestRepo(t)

	if _, err := moveFile(root, docsRoot, ".docs/guide/a.md", ".docs/guide/c.md"); err != nil {
		t.Fatal(err)
	}
	moved := readFileString(t, filepath.Join(docsRoot, "guide", "c.md"))
	if !strings.Contains(moved, "self [me](c.md#top)") {
		t.Errorf("self link not rewritten:\n%s", moved)
	}
	if !strings.Contains(moved, "[other](../notes/n.md)") {
		t.Errorf("link that still resolves was touched:\n%s", moved)
	}
	requireLinksClean(t, root, docsRoot)
}

// cinch:rule CINCH-018
func TestMoveChangingDepth(t *testing.T) {
	root, docsRoot := plansTestRepo(t, moveManifest, map[string]string{
		"plans/roadmap.md":   "# Roadmap\n[x](x.md) and [y](archive/y.md)\n",
		"plans/x.md":         "# X\n[r](roadmap.md)\n",
		"plans/archive/y.md": "# Y\n[r](../roadmap.md) and [x](../x.md)\n",
		"top.md":             "# Top\n[r](plans/roadmap.md)\n",
	})

	if _, err := moveFile(root, docsRoot, ".docs/plans/roadmap.md", ".docs/roadmap.md"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"roadmap.md":         "# Roadmap\n[x](plans/x.md) and [y](plans/archive/y.md)\n",
		"plans/x.md":         "# X\n[r](../roadmap.md)\n",
		"plans/archive/y.md": "# Y\n[r](../../roadmap.md) and [x](../x.md)\n",
		"top.md":             "# Top\n[r](roadmap.md)\n",
	}
	for name, content := range want {
		if got := readFileString(t, filepath.Join(docsRoot, filepath.FromSlash(name))); got != content {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, content)
		}
	}
	requireLinksClean(t, root, docsRoot)
}

// cinch:rule CINCH-018
func TestMoveRefusalsChangeNothing(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		target  string
		wantErr string
	}{
		{"existing target", ".docs/guide/a.md", ".docs/guide/b.md", "already exists"},
		{"target is the source", ".docs/guide/a.md", ".docs/guide/a.md", "already exists"},
		{"source outside the docs root", "outside.md", ".docs/moved.md", "outside the docs root"},
		{"target outside the docs root", ".docs/guide/a.md", "moved.md", "outside the docs root"},
		{"target climbs out of the docs root", ".docs/guide/a.md", ".docs/../moved.md", "outside the docs root"},
		{"non-markdown source", ".docs/img/d.png", ".docs/img/e.png", ".md"},
		{"non-markdown target", ".docs/guide/a.md", ".docs/guide/a.txt", ".md"},
		{"missing source", ".docs/guide/none.md", ".docs/guide/new.md", "does not exist"},
		{"directory source", ".docs/guide", ".docs/other.md", "regular file"},
		{"target parent is a file", ".docs/guide/a.md", ".docs/guide/b.md/a.md", "parent is not a directory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, docsRoot := moveTestRepo(t)
			writeTestFile(t, root, "outside.md", "# Outside\n[a](.docs/guide/a.md)\n")
			before := snapshotTree(t, root)

			_, err := moveFile(root, docsRoot, c.source, c.target)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("moveFile error = %v, want one containing %q", err, c.wantErr)
			}
			requireSameTree(t, before, snapshotTree(t, root))

			if code := CmdMv(root, c.source, c.target); code != 1 {
				t.Fatalf("CmdMv exit = %d, want 1", code)
			}
			requireSameTree(t, before, snapshotTree(t, root))
		})
	}
}

// cinch:rule CINCH-018
func TestCmdMvSucceeds(t *testing.T) {
	root, docsRoot := moveTestRepo(t)
	if code := CmdMv(root, ".docs/guide/a.md", ".docs/ref/a.md"); code != 0 {
		t.Fatalf("CmdMv exit = %d, want 0", code)
	}
	requireLinksClean(t, root, docsRoot)
}

// cinch:rule CINCH-018
func TestMoveLeavesUnresolvedLinksAlone(t *testing.T) {
	root, docsRoot := plansTestRepo(t, moveManifest, map[string]string{
		"a/x.md": "# X\n[gone](missing.md) and [up](../b/y.md) and [bad](nowhere/z.md)\n",
		"b/y.md": "# Y\n",
	})

	if _, err := moveFile(root, docsRoot, ".docs/a/x.md", ".docs/c/d/x.md"); err != nil {
		t.Fatal(err)
	}
	want := "# X\n[gone](missing.md) and [up](../../b/y.md) and [bad](nowhere/z.md)\n"
	if got := readFileString(t, filepath.Join(docsRoot, "c", "d", "x.md")); got != want {
		t.Errorf("moved file =\n%s\nwant\n%s", got, want)
	}
}
