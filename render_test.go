package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFrontMatterSelection — a template whose requires: keys are absent from
// the manifest is not rendered; one whose keys are present is. Selection is
// key presence, not a branch (D063).
func TestFrontMatterSelection(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	frag := filepath.Join(tdir, fragmentDir)
	mustMkdir(t, tdir, frag)

	write := func(p, body string) {
		if err := os.WriteFile(filepath.Join(root, "templates", p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("needs-domain.md", "---\nrequires: paths.domain\n---\n# Needs domain\n{{paths.domain}}\n")
	write("plain.md", "# Plain\nok\n")

	m := &Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
	haveDomain, err := renderAll(root, m)
	if err != nil {
		t.Fatalf("render with domain: %v", err)
	}
	if len(haveDomain) != 2 || haveDomain[0].rel != "needs-domain.md" || !strings.Contains(haveDomain[0].body, ".agent/domain") {
		t.Fatalf("with domain, expected needs-domain + plain, got %+v", haveDomain)
	}

	noDomain := &Manifest{Vars: map[string]string{}}
	without, err := renderAll(root, noDomain)
	if err != nil {
		t.Fatalf("render without domain: %v", err)
	}
	if len(without) != 1 || without[0].rel != "plain.md" {
		t.Fatalf("without domain, expected only plain, got %+v", without)
	}
}

// TestFragmentComposition — an anchor composes its fragment in when the
// fragment's requires are met and removes the anchor when they are not.
func TestFragmentComposition(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	mustMkdir(t, tdir, filepath.Join(tdir, fragmentDir))

	if err := os.WriteFile(filepath.Join(tdir, "fix.md"), []byte(
		"# Fix\n\nbase row\n<!-- compose: devservers -->\ntail row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tdir, fragmentDir, "devservers.md"), []byte(
		"---\nrequires: commands.start-backend\n---\n| Dev servers (`{{commands.start-backend}}`) | x |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fragment included when the key exists.
	withCmd, err := renderAll(root, &Manifest{Vars: map[string]string{
		"commands.start-backend": "make dev-backend",
	}})
	if err != nil {
		t.Fatalf("render with key: %v", err)
	}
	body := withCmd[0].body
	if !strings.Contains(body, "| Dev servers (`make dev-backend`) | x |") {
		t.Fatalf("fragment not composed into base:\n%s", body)
	}
	if strings.Contains(body, "compose:") {
		t.Fatalf("anchor comment survived:\n%s", body)
	}
	if !strings.Contains(body, "tail row") {
		t.Fatalf("base after anchor lost:\n%s", body)
	}

	// Fragment skipped when the key is absent; the anchor vanishes, the base
	// stays complete.
	withoutCmd, err := renderAll(root, &Manifest{Vars: map[string]string{}})
	if err != nil {
		t.Fatalf("render without key: %v", err)
	}
	body = withoutCmd[0].body
	if strings.Contains(body, "Dev servers") {
		t.Fatalf("fragment rendered without its key:\n%s", body)
	}
	if strings.Contains(body, "compose:") {
		t.Fatalf("anchor comment survived without fragment:\n%s", body)
	}
	if !strings.Contains(body, "tail row") {
		t.Fatalf("base after anchor lost without fragment:\n%s", body)
	}
}

// TestAnchorWithoutFragmentIsAnError — a marker naming a fragment file that
// does not exist is an authoring mistake, not a silent no-op.
func TestAnchorWithoutFragmentIsAnError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	mustMkdir(t, tdir, filepath.Join(tdir, fragmentDir))

	if err := os.WriteFile(filepath.Join(tdir, "fix.md"), []byte("# Fix\n<!-- compose: ghost -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := renderAll(root, &Manifest{Vars: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected missing-fragment error, got %v", err)
	}
}

// TestFragmentNeverRendersStandalone — fragments/ is skipped by the walker.
func TestFragmentNeverRendersStandalone(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	mustMkdir(t, tdir, filepath.Join(tdir, fragmentDir))

	if err := os.WriteFile(filepath.Join(tdir, "fix.md"), []byte("# Fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tdir, fragmentDir, "devservers.md"), []byte("dev row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(root, &Manifest{Vars: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].rel != "fix.md" {
		t.Fatalf("fragment rendered standalone: %+v", files)
	}
}

func mustMkdir(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
