package cinch

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(manifestPath)), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(manifestPath), err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestPath), []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestLoadManifest_ParsesKeyValue(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "# comment\nfoo.bar = baz\ncommands.check = make check\n\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars["foo.bar"] != "baz" {
		t.Fatalf("foo.bar: want %q, got %q", "baz", m.Vars["foo.bar"])
	}
	if m.Vars["commands.check"] != "make check" {
		t.Fatalf("commands.check: want %q, got %q", "make check", m.Vars["commands.check"])
	}
	if len(m.Vars) != 2 {
		t.Fatalf("want 2 vars (comment/blank lines skipped), got %v", m.Vars)
	}
}

func TestLoadManifest_MissingFileFires(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with no .docs/manifest: want error, got nil")
	}
}

func TestLoadManifest_MalformedLineFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "not a key value line\n")

	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with malformed line: want error, got nil")
	}
}

func TestLoadManifestOptional_MissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	m, err := loadManifestOptional(dir)
	if err != nil {
		t.Fatalf("loadManifestOptional with no .docs/manifest: want nil error, got %v", err)
	}
	if m != nil {
		t.Fatalf("loadManifestOptional with no .docs/manifest: want nil manifest, got %v", m)
	}
}

func TestLoadManifestOptional_MalformedLineFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "not a key value line\n")

	if _, err := loadManifestOptional(dir); err == nil {
		t.Fatalf("loadManifestOptional with malformed line: want error, got nil")
	}
}

func TestLoadManifestOptional_ParsesKeyValue(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths.docs = mydocs\n")

	m, err := loadManifestOptional(dir)
	if err != nil {
		t.Fatalf("loadManifestOptional: %v", err)
	}
	if m.Vars["paths.docs"] != "mydocs" {
		t.Fatalf("paths.docs: want %q, got %q", "mydocs", m.Vars["paths.docs"])
	}
}
