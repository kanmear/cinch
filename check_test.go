package main

import (
	"path/filepath"
	"testing"
)

func TestResolveDocsRoot_DefaultsWithNoManifest(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("resolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, ".docs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_DefaultsWithManifestButNoKey(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "commands.check = make check\n")

	got, err := resolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("resolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, ".docs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_RelativeKeyJoinsRoot(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths.docs = mydocs\n")

	got, err := resolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("resolveDocsRoot: %v", err)
	}
	want := filepath.Join(dir, "mydocs")
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveDocsRoot_AbsoluteKeyUsedAsIs(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere", "docs")
	writeManifest(t, dir, "paths.docs = "+abs+"\n")

	got, err := resolveDocsRoot(dir)
	if err != nil {
		t.Fatalf("resolveDocsRoot: %v", err)
	}
	if got != abs {
		t.Fatalf("want %q, got %q", abs, got)
	}
}

func TestResolveDocsRoot_MalformedManifestFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "not a key value line\n")

	if _, err := resolveDocsRoot(dir); err == nil {
		t.Fatalf("resolveDocsRoot with malformed manifest: want error, got nil")
	}
}
