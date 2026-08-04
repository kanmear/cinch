package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func manifestWithSeams(seams string) *Manifest {
	m := &Manifest{Raw: map[string]any{}}
	if err := yaml.Unmarshal([]byte("development:\n  commands:\n    check: x\n    docs-index: y\nseams:\n"+seams), &m.Raw); err != nil {
		panic(err)
	}
	m.Vars = map[string]string{}
	flatten("", m.Raw, m.Vars)
	return m
}

func TestCheckSeamsAllow(t *testing.T) {
	cases := []struct {
		name, seams, wantErr string
	}{
		{
			"resolves",
			"  auditor: { tier: strong, allow: [check, docs-index] }",
			"",
		},
		{
			"unknown id",
			"  auditor: { tier: strong, allow: [check, nope] }",
			`seam "auditor" allow "nope" is not a declared development.commands id`,
		},
		{
			"not a list",
			"  auditor: { tier: strong, allow: check }",
			"seam \"auditor\" allow must be a non-empty list of command ids",
		},
		{
			"empty list",
			"  auditor: { tier: strong, allow: [] }",
			"seam \"auditor\" allow must be a non-empty list of command ids",
		},
		{
			"non-string entry",
			"  auditor: { tier: strong, allow: [check, 7] }",
			"seam \"auditor\" allow entries must be strings",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := manifestWithSeams(c.seams)
			var r report
			checkSeams(m, &r)
			for _, f := range r.findings {
				if f.level == "error" && f.code == "C12" {
					if f.msg != c.wantErr {
						t.Fatalf("C12 error = %q, want %q", f.msg, c.wantErr)
					}
					return
				}
			}
			if c.wantErr != "" {
				t.Fatalf("no C12 error, want %q", c.wantErr)
			}
		})
	}
}
