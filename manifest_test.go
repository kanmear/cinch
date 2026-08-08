package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".agent"), 0o755); err != nil {
		t.Fatalf("mkdir .agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestPath), []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestLoadManifest_ParsesKeyValue(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "# comment\npaths.domain = .agent/domain\ncommands.check = make check\n\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars["paths.domain"] != ".agent/domain" {
		t.Fatalf("paths.domain: want %q, got %q", ".agent/domain", m.Vars["paths.domain"])
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
		t.Fatalf("loadManifest with no .agent/manifest: want error, got nil")
	}
}

func TestLoadManifest_MalformedLineFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "not a key value line\n")

	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with malformed line: want error, got nil")
	}
}
