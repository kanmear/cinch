package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Seeded defect battery: generated.go / header.go ---
//
// Each fixture renders a real tree via CmdRender, then perturbs it the way
// a human edit or a stale checkout would, and asserts checkGenerated
// catches it.

func renderedFixture(t *testing.T) (root string, m *manifest, files []renderFile) {
	t.Helper()
	root = t.TempDir()
	writeTestFile(t, root, "cinch.yml", "")
	if code := CmdRender(root); code != 0 {
		t.Fatalf("CmdRender = %d, want 0", code)
	}
	m = loadTestManifest(t, root)
	var err error
	files, err = renderAll(m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("renderAll returned no files")
	}
	return root, m, files
}

func TestSeededDefectGeneratedHandEdited(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	path := filepath.Join(root, target.destination)

	original := readTestFile(t, root, target.destination)
	if err := os.WriteFile(path, []byte(original+"\nhand-edited line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(root, m, nil)

	ok := false
	for _, f := range result.findings {
		if f.file == target.destination && strings.Contains(f.message, "does not match a fresh render") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'does not match a fresh render' finding for %s", result.findings, target.destination)
	}
}

func TestSeededDefectGeneratedMissingFile(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	path := filepath.Join(root, target.destination)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(root, m, nil)

	ok := false
	for _, f := range result.findings {
		if f.file == target.destination && strings.Contains(f.message, "missing — run 'cinch render'") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want a 'missing' finding for %s", result.findings, target.destination)
	}
}

func TestSeededDefectGeneratedOrphanFile(t *testing.T) {
	root, m, files := renderedFixture(t)
	target := files[0]
	dir := filepath.Dir(filepath.Join(root, target.destination))

	orphanContent := header("orphan-source.md", "orphan body", styleMarkdown) + "orphan body"
	orphanPath := filepath.Join(dir, "orphan-leftover.md")
	if err := os.WriteFile(orphanPath, []byte(orphanContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkGenerated(root, m, nil)

	ok := false
	for _, f := range result.findings {
		if strings.Contains(f.file, "orphan-leftover.md") && strings.Contains(f.message, "orphaned generated file") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("findings = %+v, want an 'orphaned generated file' finding for orphan-leftover.md", result.findings)
	}
}
