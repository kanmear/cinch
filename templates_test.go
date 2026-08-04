package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTemplatePurity checks templates/*.md for leaks that are structural — a
// shape no future harness or stack can hide from — rather than enumerating
// harness names, which rot the moment a new tool exists (D049). `/foo` is a
// leak by construction; "Claude Code" would only be a leak by lookup against
// a list that can never be complete, so no such list exists here. The
// general case — prose describing how a runner happens to surface
// something, with no flagged token — has no structural signature and stays
// a human judgment call while porting (AGENTS.md).
//
// This is deliberately not a check.go C-check: C1-C12 validate a consumer's
// rendered/bound state against cinch's contracts, and templates/ has no
// consumer-side referent (templates resolve next to the cinch binary, D047).
// This validates cinch's own source, so it's a plain test.
func TestTemplatePurity(t *testing.T) {
	// Command-shaped inline-code token: backtick, slash, one lowercase-
	// hyphenated word, backtick. The exact shape of a slash-command
	// invocation (`/execute-plan`, `/rules`) regardless of which harness
	// defines it. Anchored to the whole span, not "contains a slash", so an
	// ordinary path example like `backend/tests` doesn't match. A
	// single-segment absolute path like `/mnt` would still match this
	// pattern — a known residual false-positive risk, accepted because no
	// current template content triggers it; revisit only if it does.
	commandTokenRe := regexp.MustCompile("`/[a-z][a-z0-9-]*`")

	// Runner-config dotdir path: `.word/` or `~/.word/`, e.g. `.claude/`,
	// `.cursor/`, `~/.config/`. Excludes `.agent/` by name — cinch's own
	// portable convention (D045: workflows are referenced by
	// `.agent/workflows/<name>.md` path) — which is a single named
	// exception for the one convention this repo defines, not a growing
	// blocklist of runner names.
	dotdirRe := regexp.MustCompile(`\.([a-z][a-z0-9_-]*)/`)

	stackLiterals := commandLiterals(t)

	err := filepath.WalkDir("templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(b), "\n") {
			lineNo := i + 1

			if m := commandTokenRe.FindString(line); m != "" {
				t.Errorf("%s:%d: command-shaped token %s — reference the workflow by "+
					"its .agent/workflows/<name>.md path instead", p, lineNo, m)
			}

			for _, m := range dotdirRe.FindAllStringSubmatch(line, -1) {
				if m[1] == "agent" {
					continue
				}
				t.Errorf("%s:%d: runner-config-shaped path %q — templates are harness-neutral",
					p, lineNo, m[0])
			}

			for _, lit := range stackLiterals {
				if lit != "" && strings.Contains(line, lit) {
					t.Errorf("%s:%d: hardcoded command %q from scaffold/manifest.example.yml — "+
						"reference it as {{commands.<key>}} instead", p, lineNo, lit)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking templates: %v", err)
	}
}

// commandLiterals returns every string value under development.commands in
// the scaffold example — the variable contract templates are written
// against (D040/D045). Not loadManifest/agentDir: cinch's own repo has no
// .agent/manifest.yml, the scaffold example is the contract here.
func commandLiterals(t *testing.T) []string {
	b, err := os.ReadFile(filepath.Join("scaffold", "manifest.example.yml"))
	if err != nil {
		t.Fatalf("reading scaffold/manifest.example.yml: %v", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		t.Fatalf("parsing scaffold/manifest.example.yml: %v", err)
	}
	dev, _ := raw["development"].(map[string]any)
	cmds, _ := dev["commands"].(map[string]any)

	out := make([]string, 0, len(cmds))
	for _, v := range cmds {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
