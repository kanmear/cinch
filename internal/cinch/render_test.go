package cinch

import (
	"strings"
	"testing"
)

func TestSubstitute_KnownVariableSubstitutes(t *testing.T) {
	out, missing := substitute("see {{paths.domain}}/overview.md", map[string]string{"paths.domain": ".agent/domain"})
	if len(missing) != 0 {
		t.Fatalf("want no missing keys, got %v", missing)
	}
	want := "see .agent/domain/overview.md"
	if out != want {
		t.Fatalf("substitute: want %q, got %q", want, out)
	}
}

func TestSubstitute_CommandsVariableSubstitutes(t *testing.T) {
	out, missing := substitute("run `{{commands.check}}` first", map[string]string{"commands.check": "make check"})
	if len(missing) != 0 {
		t.Fatalf("want no missing keys, got %v", missing)
	}
	want := "run `make check` first"
	if out != want {
		t.Fatalf("substitute: want %q, got %q", want, out)
	}
}

func TestSubstitute_UndefinedVariableFires(t *testing.T) {
	out, missing := substitute("see {{paths.domain}}/x.md and {{commands.check}}", nil)
	if len(missing) != 2 {
		t.Fatalf("want 2 missing keys, got %v", missing)
	}
	if missing[0] != "commands.check" || missing[1] != "paths.domain" {
		t.Fatalf("want sorted [commands.check paths.domain], got %v", missing)
	}
	want := "see {{paths.domain}}/x.md and {{commands.check}}"
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
	a := header("templates/rules.md", "same body")
	b := header("templates/rules.md", "same body")
	if a != b {
		t.Fatalf("header: want identical output for identical input, got %q vs %q", a, b)
	}
}

func TestHeader_DifferentBodyProducesDifferentHash(t *testing.T) {
	a := header("templates/rules.md", "body one")
	b := header("templates/rules.md", "body two")
	if a == b {
		t.Fatalf("header: want different hash for different body, got identical %q", a)
	}
}

func TestRenderAll_PhilosophyIsCopiedVerbatim(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		if f.Dest == ".agent/workflows/doc-philosophy.md" {
			if f.Body != philosophySrc {
				t.Fatalf("philosophy body was not copied verbatim")
			}
			if strings.Contains(f.Body, "{{") {
				t.Fatalf("philosophy.md still has an unresolved {{}} token — it must need no renderer")
			}
			return
		}
	}
	t.Fatalf("renderAll: doc-philosophy.md not produced")
}

func TestRenderAll_UndefinedVariableFires(t *testing.T) {
	if _, err := renderAll(&Manifest{Vars: nil}); err == nil {
		t.Fatalf("renderAll with no paths.domain: want error, got nil")
	}
}

func TestRenderAll_IdempotentReRender(t *testing.T) {
	m := &Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
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
		if header(first[i].Source, first[i].Body) != header(second[i].Source, second[i].Body) {
			t.Fatalf("header: not idempotent for %q", first[i].Dest)
		}
	}
}
