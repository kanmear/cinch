package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"cinch/internal/manifest"
	"cinch/internal/render"
)

func manifestWithSeams(seams string) *manifest.Manifest {
	m := &manifest.Manifest{Raw: map[string]any{}}
	if err := yaml.Unmarshal([]byte("development:\n  commands:\n    check: x\n    docs-index: y\nseams:\n"+seams), &m.Raw); err != nil {
		panic(err)
	}
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

// TestCheckRenderedTamper — a committed file whose body no longer matches its
// own header hash is hand-edited, not stale. The marker for HARNESS-004 sits
// above checkRendered (check.go), the C12 precedent (TestCheckSeamsAllow).
func TestCheckRenderedTamper(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	mustMkdir(t, tdir)

	if err := os.WriteFile(filepath.Join(tdir, "w.md"), []byte("# W\n{{paths.domain}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
	files, err := render.RenderAll(root, m)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	pristine := files[0].Body

	dst := filepath.Join(root, ".agent", "workflows", "w.md")
	mustMkdir(t, filepath.Dir(dst))
	// Hand-edit: the header still carries the hash of the pristine body, but
	// the body no longer matches it.
	tampered := strings.Replace(pristine, "domain", "domain ", 1)
	if tampered == pristine {
		t.Fatal("test edit did not change the body")
	}
	if err := os.WriteFile(dst, []byte(render.Header("w.md", pristine)+tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	var r report
	checkRendered(root, m, &r)
	msg := c2Message(r)
	if !strings.Contains(msg, "hand-edited") {
		t.Fatalf("C2 = %q, want a hand-edited finding", msg)
	}
	if strings.Contains(msg, "stale") {
		t.Fatalf("C2 = %q, tamper misreported as staleness", msg)
	}
}

// TestCheckRenderedStale — a committed file whose header hash matches its own
// body but whose body differs from a fresh render is stale, not tampered.
func TestCheckRenderedStale(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CINCH_HOME", root)
	tdir := filepath.Join(root, "templates")
	mustMkdir(t, tdir)

	if err := os.WriteFile(filepath.Join(tdir, "w.md"), []byte("# W\n{{paths.domain}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := &manifest.Manifest{Vars: map[string]string{"paths.domain": ".agent/domain"}}
	files, err := render.RenderAll(root, old)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}

	dst := filepath.Join(root, ".agent", "workflows", "w.md")
	mustMkdir(t, filepath.Dir(dst))
	// Committed as rendered under the old manifest: header and body agree.
	if err := os.WriteFile(dst, []byte(render.Header("w.md", files[0].Body)+files[0].Body), 0o644); err != nil {
		t.Fatal(err)
	}

	// The manifest moves on; the committed file does not.
	fresh := &manifest.Manifest{Vars: map[string]string{"paths.domain": ".agent/harness"}}
	var r report
	checkRendered(root, fresh, &r)
	msg := c2Message(r)
	if !strings.Contains(msg, "stale against the manifest") {
		t.Fatalf("C2 = %q, want a staleness finding", msg)
	}
	if strings.Contains(msg, "hand-edited") {
		t.Fatalf("C2 = %q, staleness misreported as tamper", msg)
	}
}

// c2Message returns the first C2 error message from a report (empty if none).
func c2Message(r report) string {
	for _, f := range r.findings {
		if f.level == "error" && f.code == "C2" {
			return f.msg
		}
	}
	return ""
}

func mustMkdir(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
