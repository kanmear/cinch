package cinch

import (
	"path/filepath"
	"testing"
)

func TestResolveDocsRoot_DefaultsWithNoManifest(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("ResolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, ".docs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_DefaultsWithManifestButNoKey(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "commands:\n  check: make check\n")

	got, err := ResolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("ResolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, ".docs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_RelativeKeyJoinsRoot(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths:\n  docs: mydocs\n")

	got, err := ResolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("ResolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, "mydocs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_AbsoluteKeyUsedAsIs(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere", "docs")
	writeManifest(t, dir, "paths:\n  docs: "+abs+"\n")

	got, err := ResolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("ResolveDocsRoot: %v", err)
	}
	if got != abs {
		t.Fatalf("want %q, got %q", abs, got)
	}
}

func TestResolveDocsRoot_MalformedManifestFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "foo: [unterminated\n")

	if _, err := ResolveDocsRoot(dir); err == nil {
		t.Fatalf("ResolveDocsRoot with malformed manifest: want error, got nil")
	}
}
