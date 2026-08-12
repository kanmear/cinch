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

func TestLoadManifest_ParsesNestedMapping(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "# comment\nfoo:\n  bar: baz\ncommands:\n  check: make check\n")

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
		t.Fatalf("want 2 vars (comment lines skipped), got %v", m.Vars)
	}
}

func TestLoadManifest_MissingFileFires(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with no cinch.yml: want error, got nil")
	}
}

func TestLoadManifest_MalformedYAMLFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "foo: [unterminated\n")

	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with malformed YAML: want error, got nil")
	}
}

func TestLoadManifest_NonMappingRootFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "just a scalar\n")

	if _, err := loadManifest(dir); err == nil {
		t.Fatalf("loadManifest with non-mapping root: want error, got nil")
	}
}

func TestLoadManifestOptional_MissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	m, err := loadManifestOptional(dir)
	if err != nil {
		t.Fatalf("loadManifestOptional with no cinch.yml: want nil error, got %v", err)
	}
	if m != nil {
		t.Fatalf("loadManifestOptional with no cinch.yml: want nil manifest, got %v", m)
	}
}

func TestLoadManifestOptional_MalformedYAMLFires(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "foo: [unterminated\n")

	if _, err := loadManifestOptional(dir); err == nil {
		t.Fatalf("loadManifestOptional with malformed YAML: want error, got nil")
	}
}

func TestLoadManifestOptional_ParsesNestedMapping(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths:\n  docs: mydocs\n")

	m, err := loadManifestOptional(dir)
	if err != nil {
		t.Fatalf("loadManifestOptional: %v", err)
	}
	if m.Vars["paths.docs"] != "mydocs" {
		t.Fatalf("paths.docs: want %q, got %q", "mydocs", m.Vars["paths.docs"])
	}
}

func TestLoadManifest_DuplicateKeyLastWinsFirstSeenOrder(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "hooks:\n  pre-commit:\n    a:\n      run: first\n    b:\n      run: only\n    a:\n      run: second\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars["hooks.pre-commit.a.run"] != "second" {
		t.Fatalf("hooks.pre-commit.a.run: want last value %q, got %q", "second", m.Vars["hooks.pre-commit.a.run"])
	}
	got := m.Names("hooks.pre-commit")
	want := []string{"a", "b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Names: want first-seen order %v, got %v", want, got)
	}
}

func TestLoadManifest_ReadsFromRepoRootNotDocsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cinch.yml"), []byte("paths:\n  docs: mydocs\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Vars["paths.docs"] != "mydocs" {
		t.Fatalf("paths.docs: want %q, got %q", "mydocs", m.Vars["paths.docs"])
	}

	// A manifest nested under paths.docs (the old .docs/manifest location)
	// must NOT be picked up — its location no longer depends on paths.docs.
	nested := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nested, "mydocs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "mydocs", "manifest"), []byte("paths:\n  docs: mydocs\n"), 0o644); err != nil {
		t.Fatalf("write nested manifest: %v", err)
	}
	if _, err := loadManifest(nested); err == nil {
		t.Fatalf("loadManifest: want error when manifest only exists nested under paths.docs, got nil")
	}
}

func TestManifestList_SplitsTrimsAndDropsEmpties(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "hooks:\n  pre-commit:\n    error-codes:\n      when: \"frontend/src/lib/api/ , backend/errors/ ,, \"\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	got := m.List("hooks.pre-commit.error-codes.when")
	want := []string{"frontend/src/lib/api/", "backend/errors/"}
	if len(got) != len(want) {
		t.Fatalf("List: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List: want %v, got %v", want, got)
		}
	}
}

func TestManifestList_YAMLSequenceJoinsAndSplits(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "hooks:\n  pre-commit:\n    error-codes:\n      when: [frontend/src/lib/api/, backend/errors/]\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	got := m.List("hooks.pre-commit.error-codes.when")
	want := []string{"frontend/src/lib/api/", "backend/errors/"}
	if len(got) != len(want) {
		t.Fatalf("List: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List: want %v, got %v", want, got)
		}
	}
}

func TestManifestList_MissingKeyReturnsNil(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths:\n  docs: .docs\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	if got := m.List("no.such.key"); got != nil {
		t.Fatalf("List of missing key: want nil, got %v", got)
	}
}

func TestManifestNames_DeclarationOrderAndDistinct(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir,
		"hooks:\n"+
			"  pre-commit:\n"+
			"    error-codes:\n"+
			"      run: scripts/check_error_codes.sh\n"+
			"      when: [frontend/src/lib/api/, backend/errors/]\n"+
			"    frontend:\n"+
			"      run: make check-frontend\n"+
			"      when: [frontend/]\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	got := m.Names("hooks.pre-commit")
	want := []string{"error-codes", "frontend"}
	if len(got) != len(want) {
		t.Fatalf("Names: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names: want %v (declaration order), got %v", want, got)
		}
	}
}

func TestManifestNames_NoMatchReturnsNil(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "paths:\n  docs: .docs\n")

	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	if got := m.Names("hooks.pre-commit"); got != nil {
		t.Fatalf("Names with no matches: want nil, got %v", got)
	}
}
