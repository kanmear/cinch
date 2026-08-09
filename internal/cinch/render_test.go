package cinch

import (
	"io/fs"
	"path/filepath"
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

func TestRenderAll_RendersEveryTemplatePlusPhilosophyAndIndex(t *testing.T) {
	entries, err := fs.ReadDir(templatesFS, templatesDir)
	if err != nil {
		t.Fatalf("reading embedded templates: %v", err)
	}
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	want := len(entries) + 2 // + doc-philosophy.md + index.md
	if len(files) != want {
		t.Fatalf("renderAll: want %d files (one per template + philosophy + index), got %d", want, len(files))
	}
}

func TestRenderAll_EveryTemplateFullySubstitutes(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		if strings.Contains(f.Body, "{{") {
			t.Fatalf("%s: unresolved {{}} token in rendered body", f.Dest)
		}
	}
}

func TestRenderAll_UndefinedVariableNamesEveryAffectedTemplate(t *testing.T) {
	_, err := renderAll(&Manifest{Vars: nil})
	if err == nil {
		t.Fatalf("renderAll with no paths.domain: want error, got nil")
	}
	msg := err.Error()
	// paths.domain is used by more than one template — the error must name
	// each affected template, not just the first one found.
	count := strings.Count(msg, "paths.domain")
	if count < 2 {
		t.Fatalf("renderAll error: want paths.domain named for multiple templates, got %d mention(s):\n%s", count, msg)
	}
	if !strings.Contains(msg, "templates/check-rules.md") || !strings.Contains(msg, "templates/rules.md") {
		t.Fatalf("renderAll error: want both templates/check-rules.md and templates/rules.md named:\n%s", msg)
	}
}

func TestTitleAndTrigger(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantTitle   string
		wantTrigger string
	}{
		{
			name:        "plain sentence trigger",
			body:        "# Domain Rule Maintenance\n\nAdd, edit, or reorganize domain rules in `.agent/domain/`.\n\n## Usage\n",
			wantTitle:   "Domain Rule Maintenance",
			wantTrigger: "Add, edit, or reorganize domain rules in `.agent/domain/`.",
		},
		{
			name:        "bold-prefixed trigger, no blank line before it",
			body:        "# Philosophy: Lean, Scalable Documentation\n**The Core Insight:** With clean architecture, the code IS the documentation.\n",
			wantTitle:   "Philosophy: Lean, Scalable Documentation",
			wantTrigger: "**The Core Insight:** With clean architecture, the code IS the documentation.",
		},
		{
			name:        "no H1 at all",
			body:        "no title here\njust text\n",
			wantTitle:   "",
			wantTrigger: "",
		},
		{
			name:        "sentence hand-wrapped across physical lines joins",
			body:        "# Plan Execution Workflow\n\nExecute the tasks in an already-decomposed plan file — one atomic task per session, git\nconventions enforced, and a Session Handoff trail so work resumes cold.\n\n---\n",
			wantTitle:   "Plan Execution Workflow",
			wantTrigger: "Execute the tasks in an already-decomposed plan file — one atomic task per session, git conventions enforced, and a Session Handoff trail so work resumes cold.",
		},
		{
			name:        "stops before a bullet immediately following, no blank line between",
			body:        "# Documentation Sync\n\nUpdate `.agent/` documentation to reflect recent code changes.\n- **Note:** more detail here.\n",
			wantTitle:   "Documentation Sync",
			wantTrigger: "Update `.agent/` documentation to reflect recent code changes.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, trigger := titleAndTrigger(tt.body)
			if title != tt.wantTitle {
				t.Fatalf("titleAndTrigger: title want %q, got %q", tt.wantTitle, title)
			}
			if trigger != tt.wantTrigger {
				t.Fatalf("titleAndTrigger: trigger want %q, got %q", tt.wantTrigger, trigger)
			}
		})
	}
}

func TestRenderAll_IndexListsEveryOtherWorkflow(t *testing.T) {
	files, err := renderAll(&Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}})
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	var index *renderFile
	for i := range files {
		if files[i].Dest == ".agent/workflows/index.md" {
			index = &files[i]
		}
	}
	if index == nil {
		t.Fatalf("renderAll: index.md not produced")
	}
	for _, f := range files {
		if f.Dest == index.Dest {
			continue
		}
		name := filepath.Base(f.Dest)
		if !strings.Contains(index.Body, name) {
			t.Fatalf("index.md: missing entry for %s:\n%s", name, index.Body)
		}
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
