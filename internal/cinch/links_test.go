package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findingsForCheck(findings []Finding, check string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

// marker returns a // cinch:rule line for id, composed at runtime so this
// file's own source carries no literal marker for the scan to find.
func marker(id string) string { return "// cinch:" + "rule " + id }

func TestLinks_BrokenLinkFires(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "a.md"), "see [x](./missing.md) for details\n")

	findings := checkLinks(docs)

	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Check != "links" || f.Level != "error" || f.Line != 1 {
		t.Fatalf("unexpected finding: %+v", f)
	}
}

func TestLinks_ResolvingLinkIsClean(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	writeFile(t, filepath.Join(docs, "a.md"), "see [x](./present.md) for details\n")
	writeFile(t, filepath.Join(docs, "present.md"), "target\n")

	findings := checkLinks(docs)

	if len(findings) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestLinks_SkipsFencedCodeAnchorsURLsMailto(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, ".docs")
	content := "" +
		"external: [x](https://example.com/missing)\n" +
		"anchor: [x](#some-heading)\n" +
		"mail: [x](mailto:nobody@example.com)\n" +
		"```\n" +
		"fenced: [x](./also-missing.md)\n" +
		"```\n"
	writeFile(t, filepath.Join(docs, "a.md"), content)

	findings := checkLinks(docs)

	if len(findings) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestLinks_MissingDocsDirIsQuiet(t *testing.T) {
	root := t.TempDir()

	findings := checkLinks(filepath.Join(root, ".docs"))

	if len(findings) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(findings), findings)
	}
}
