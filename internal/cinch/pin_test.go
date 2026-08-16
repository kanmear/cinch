package cinch

import (
	"path/filepath"
	"strings"
	"testing"
)

const testPinManifest = "require:\n  cinch: 0.1.0\n"

func TestCheckPin_DeclaredMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testPinManifest)

	if got := checkPin(root, "0.1.0"); len(got.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", got)
	}
}

func TestCheckPin_DeclaredMismatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testPinManifest)

	got := checkPin(root, "0.2.0")
	if len(got.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got.Findings), got)
	}
	f := got.Findings[0]
	if f.Check != "core" || f.Level != "error" {
		t.Fatalf("unexpected finding: %+v", f)
	}
	if !strings.Contains(f.Message, "0.2.0") || !strings.Contains(f.Message, "0.1.0") {
		t.Fatalf("finding doesn't name both versions: %+v", f)
	}
}

func TestCheckPin_Absent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")

	got := checkPin(root, "0.1.0")
	if len(got.Findings) != 0 {
		t.Fatalf("want 0 findings with no require.cinch key, got %+v", got)
	}
	if got.NoOp == "" {
		t.Fatalf("want a stated no-op reason with no require.cinch key, got none")
	}
}

func TestCheckPin_NoManifestIsNoOp(t *testing.T) {
	root := t.TempDir()

	got := checkPin(root, "0.1.0")
	if len(got.Findings) != 0 {
		t.Fatalf("want 0 findings with no manifest, got %+v", got)
	}
	if got.NoOp == "" {
		t.Fatalf("want a stated no-op reason with no manifest, got none")
	}
}

func TestCheckPin_DevelSkips(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testPinManifest)

	got := checkPin(root, "devel")
	if len(got.Findings) != 0 {
		t.Fatalf("want 0 findings for a devel binary, got %+v", got)
	}
	if got.NoOp == "" {
		t.Fatalf("want a stated no-op reason for a devel binary, got none")
	}
}

func TestSemverEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.1.0", "0.1.0", true},
		{"0.1.0", "0.2.0", false},
		{"1.2", "1.2.0", true},
		{"01.2.0", "1.2.0", true},
		{"v0.1.0", "0.1.0", false}, // "v" prefix fails to parse — falls back to string equality
		{"v0.1.0", "v0.1.0", true},
	}
	for _, c := range cases {
		if got := semverEqual(c.a, c.b); got != c.want {
			t.Fatalf("semverEqual(%q, %q): want %v, got %v", c.a, c.b, c.want, got)
		}
	}
}
