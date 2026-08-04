package main

import (
	"strings"
	"testing"
)

func TestDocStructsFromText(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []docStruct
	}{
		{
			name: "single struct with tags, dash, and untagged",
			doc: `# User Model

## Go Struct

` + "```go" + `
type User struct {
	ID           int    ` + "`json:\"id\"`" + `
	Name         string ` + "`json:\"name\"`" + `
	PasswordHash string ` + "`json:\"-\"`" + `
	CreatedAt    time.Time
}
` + "```" + `
`,
			want: []docStruct{{name: "User", fields: []string{"id", "name", "PasswordHash", "CreatedAt"}}},
		},
		{
			name: "multiple structs, each in its own fence",
			doc: `# Signature Model

## SignatureSigner Struct

` + "```go" + `
type SignatureSigner struct {
	ID          int     ` + "`json:\"id\"`" + `
	DeclineNote *string ` + "`json:\"decline_note\"`" + `
}
` + "```" + `

## TabSignature Struct

` + "```go" + `
type SignatureStatus string

type TabSignature struct {
	ID     int               ` + "`json:\"id\"`" + `
	Signers []SignatureSigner ` + "`json:\"signers\"`" + `
}
` + "```" + `
`,
			want: []docStruct{
				{name: "SignatureSigner", fields: []string{"id", "decline_note"}},
				{name: "TabSignature", fields: []string{"id", "signers"}},
			},
		},
		{
			name: "no fence",
			doc:  "# User Model\n\nJust prose, no code.\n",
			want: nil,
		},
		{
			name: "comments and blank lines inside struct",
			doc: "```go\ntype Foo struct {\n\t// leading comment\n\tA int `json:\"a\"` // trailing comment\n\n\tB string\n}\n```\n",
			want: []docStruct{{name: "Foo", fields: []string{"a", "B"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docStructsFromText(tc.doc)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d structs, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i].name != tc.want[i].name {
					t.Errorf("struct %d: name %q, want %q", i, got[i].name, tc.want[i].name)
				}
				if strings.Join(got[i].fields, ",") != strings.Join(tc.want[i].fields, ",") {
					t.Errorf("struct %d (%s): fields %v, want %v", i, got[i].name, got[i].fields, tc.want[i].fields)
				}
			}
		})
	}
}

func TestRouteHeadingRe(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"## POST /api/auth/signup", true},
		{"## GET /api/tabs", true},
		{"## Go Struct", false},
		{"### POST /api/tabs", false},
		{"## post /api/tabs", false},
		{"## PUT /api/projects/{id}", true},
		{"# POST /api/tabs", false},
	}
	for _, tc := range cases {
		if got := routeHeadingRe.MatchString(tc.line); got != tc.want {
			t.Errorf("routeHeadingRe(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}
