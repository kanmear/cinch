package cinch

import (
	"reflect"
	"testing"
)

func parseTestManifest(t *testing.T, body string) *Manifest {
	t.Helper()
	m, err := parseManifestBytes([]byte(body), "test")
	if err != nil {
		t.Fatalf("parseManifestBytes: %v", err)
	}
	return m
}

func TestManifestListKeepsCommasInItems(t *testing.T) {
	m := parseTestManifest(t, "hooks:\n  pre-commit:\n    lint:\n      when:\n        - \"src/foo,bar.py\"\n        - src/baz.py\n")
	got := m.List("hooks.pre-commit.lint.when")
	want := []string{"src/foo,bar.py", "src/baz.py"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestManifestListPreservesOrder(t *testing.T) {
	m := parseTestManifest(t, "hooks:\n  pre-commit:\n    lint:\n      when: [a, b, c]\n")
	got := m.List("hooks.pre-commit.lint.when")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestManifestListEmptyMissingAndScalar(t *testing.T) {
	m := parseTestManifest(t, "commit:\n  pattern: 'x, y'\nhooks:\n  pre-commit:\n    lint:\n      when: []\n")
	if got := m.List("hooks.pre-commit.lint.when"); len(got) != 0 {
		t.Fatalf("empty when list = %v, want length 0", got)
	}
	if got := m.List("hooks.pre-commit.missing.when"); got != nil {
		t.Fatalf("missing when list = %v, want nil", got)
	}
	if got := m.List("commit.pattern"); got != nil {
		t.Fatalf("scalar key via List = %v, want nil", got)
	}
	if got, ok := m.Vars["commit.pattern"]; !ok || got != "x, y" {
		t.Fatalf("scalar comma value = %q, want %q intact", got, "x, y")
	}
}

func TestManifestListRejectsNonScalarItems(t *testing.T) {
	_, err := parseManifestBytes([]byte("hooks:\n  pre-commit:\n    lint:\n      when: [{a: 1}]\n"), "test")
	if err == nil {
		t.Fatal("expected error for non-scalar list item")
	}
}

func TestManifestNamesIncludesListOnlyEntry(t *testing.T) {
	m := parseTestManifest(t, "hooks:\n  pre-commit:\n    lint:\n      when: [src/a]\n")
	got := m.Names("hooks.pre-commit")
	want := []string{"lint"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names = %v, want %v", got, want)
	}
}

func TestListVarSubstitutesCommaJoined(t *testing.T) {
	m := parseTestManifest(t, "hooks:\n  pre-commit:\n    lint:\n      when: [src/a, src/b]\n")
	vars := substitutionVars(m)
	body, missing := substitute("when={{hooks.pre-commit.lint.when}}", vars)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing vars: %v", missing)
	}
	if body != "when=src/a, src/b" {
		t.Fatalf("substituted body = %q, want %q", body, "when=src/a, src/b")
	}
}