package cinch

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestSubstitute_KnownVariableSubstitutes(t *testing.T) {
	out, missing := substitute("see {{some.key}}/overview.md", map[string]string{"some.key": "some/value"})
	if len(missing) != 0 {
		t.Fatalf("want no missing keys, got %v", missing)
	}
	want := "see some/value/overview.md"
	if out != want {
		t.Fatalf("substitute: want %q, got %q", want, out)
	}
}

func TestSubstitute_CommandsVariableSubstitutes(t *testing.T) {
	out, missing := substitute("run `{{other.key}}` first", map[string]string{"other.key": "make check"})
	if len(missing) != 0 {
		t.Fatalf("want no missing keys, got %v", missing)
	}
	want := "run `make check` first"
	if out != want {
		t.Fatalf("substitute: want %q, got %q", want, out)
	}
}

func TestSubstitute_UndefinedVariableFires(t *testing.T) {
	out, missing := substitute("see {{some.key}}/x.md and {{other.key}}", nil)
	if len(missing) != 2 {
		t.Fatalf("want 2 missing keys, got %v", missing)
	}
	if missing[0] != "other.key" || missing[1] != "some.key" {
		t.Fatalf("want sorted [other.key some.key], got %v", missing)
	}
	want := "see {{some.key}}/x.md and {{other.key}}"
	if out != want {
		t.Fatalf("substitute: want tokens left intact %q, got %q", want, out)
	}
}

func TestSubstitute_NoVariablesIsUnchanged(t *testing.T) {
	body := "no tokens here, verbatim content"
	out, missing := substitute(body, nil)
	if len(missing) != 0 {
		t.Fatalf("want no missing keys, got %v", missing)
	}
	if out != body {
		t.Fatalf("substitute: want unchanged body, got %q", out)
	}
}

func TestHeader_IdenticalInputsProduceIdenticalHeader(t *testing.T) {
	a := header("docs/templates/docs-maintain-domain.md", "same body", styleMarkdown)
	b := header("docs/templates/docs-maintain-domain.md", "same body", styleMarkdown)
	if a != b {
		t.Fatalf("header: want identical output for identical input, got %q vs %q", a, b)
	}
}

func TestHeader_DifferentBodyProducesDifferentHash(t *testing.T) {
	a := header("docs/templates/docs-maintain-domain.md", "body one", styleMarkdown)
	b := header("docs/templates/docs-maintain-domain.md", "body two", styleMarkdown)
	if a == b {
		t.Fatalf("header: want different hash for different body, got identical %q", a)
	}
}

func TestRenderAll_PhilosophyIsCopiedVerbatim(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.docs": ".docs"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		if f.Dest == ".docs/workflows/docs-philosophy.md" {
			if f.Body != philosophySrc {
				t.Fatalf("philosophy body was not copied verbatim")
			}
			if strings.Contains(f.Body, "{{") {
				t.Fatalf("philosophy.md still has an unresolved {{}} token — it must need no renderer")
			}
			return
		}
	}
	t.Fatalf("renderAll: docs-philosophy.md not produced")
}

func TestRenderAll_EmptyManifestDefaultsPathsDocs(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: nil})
	if err != nil {
		t.Fatalf("renderAll with empty manifest: want success (paths.docs defaults to %q), got %v", defaultDocsPath, err)
	}
	for _, f := range files {
		if f.Dest == ".docs/workflows/docs-maintain-domain.md" {
			if !strings.Contains(f.Body, defaultDocsPath) {
				t.Fatalf("docs-maintain-domain.md: want default %q substituted, got:\n%s", defaultDocsPath, f.Body)
			}
			return
		}
	}
	t.Fatalf("renderAll: docs-maintain-domain.md not produced")
}

func TestRenderAll_RendersEveryTemplatePlusPhilosophyAndShims(t *testing.T) {
	entries, err := fs.ReadDir(templatesFS, templatesDir)
	if err != nil {
		t.Fatalf("reading embedded templates: %v", err)
	}
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.docs": ".docs"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	want := len(entries) + 3 // + docs-philosophy.md + 2 hook shims
	if len(files) != want {
		t.Fatalf("renderAll: want %d files (one per template + philosophy + hook shims), got %d", want, len(files))
	}
}

func TestRenderAll_EveryTemplateFullySubstitutes(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.docs": ".docs"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		if strings.Contains(f.Body, "{{") {
			t.Fatalf("%s: unresolved {{}} token in rendered body", f.Dest)
		}
	}
}

var stackSpecificPathRe = regexp.MustCompile(`\{\{paths\.docs\}\}/(backend|frontend|api|models)`)

// TestTemplates_NoStackSpecificPaths guards against a shipped template
// re-hardcoding one consumer's directory layout (deltadocs' backend/,
// frontend/, api/, models/) as if every consumer had the same structure.
// Workflow prose must route through `cinch docs` instead.
func TestTemplates_NoStackSpecificPaths(t *testing.T) {
	entries, err := fs.ReadDir(templatesFS, templatesDir)
	if err != nil {
		t.Fatalf("reading embedded templates: %v", err)
	}
	for _, e := range entries {
		src := templatesDir + "/" + e.Name()
		raw, err := fs.ReadFile(templatesFS, src)
		if err != nil {
			t.Fatalf("reading %s: %v", src, err)
		}
		if m := stackSpecificPathRe.FindString(string(raw)); m != "" {
			t.Fatalf("%s: stack-specific path %q — route through `cinch docs` instead", src, m)
		}
	}
}

func TestRenderAll_IdempotentReRender(t *testing.T) {
	m := &Manifest{Vars: map[string]string{"paths.docs": ".docs"}}
	first, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll (first): %v", err)
	}
	second, err := renderAll(m)
	if err != nil {
		t.Fatalf("renderAll (second): %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("renderAll: file count changed between runs: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("renderAll: not idempotent at %q", first[i].Dest)
		}
		if header(first[i].Source, first[i].Body, first[i].Style) != header(second[i].Source, second[i].Body, second[i].Style) {
			t.Fatalf("header: not idempotent for %q", first[i].Dest)
		}
	}
}
