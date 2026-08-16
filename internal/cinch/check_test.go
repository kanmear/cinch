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

// Replaces TestResolveDocsRoot_AbsoluteKeyUsedAsIs, which asserted the
// behavior this refusal removes. An absolute value was honored here but
// joined onto the repo root by render, so the two resolutions pointed at
// different directories and every check passed against a corpus none of them
// scanned. The refusal has to reach ResolveDocsRoot's callers, not just
// loadManifest's.
func TestResolveDocsRoot_NonLocalKeyFires(t *testing.T) {
	for _, val := range []string{filepath.Join(t.TempDir(), "elsewhere", "docs"), "../elsewhere"} {
		dir := t.TempDir()
		writeManifest(t, dir, "paths:\n  docs: "+val+"\n")

		if _, err := ResolveDocsRoot(dir); err == nil {
			t.Fatalf("ResolveDocsRoot with paths.docs = %q: want error, got nil", val)
		}
	}
}

func TestResolveDocsRoot_MalformedManifestFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "foo: [unterminated\n")

	if _, err := ResolveDocsRoot(dir); err == nil {
		t.Fatalf("ResolveDocsRoot with malformed manifest: want error, got nil")
	}
}
