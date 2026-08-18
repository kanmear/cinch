package cinch

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cinch/internal/output"
)

//go:embed docs/docs-philosophy.md
var philosophySrc string

//go:embed docs/templates
var templatesFS embed.FS

const templatesDir = "docs/templates"

var varRe = regexp.MustCompile(`\{\{([a-zA-Z0-9_.-]+)\}\}`)

func substitute(body string, vars map[string]string) (string, []string) {
	missing := map[string]bool{}
	out := varRe.ReplaceAllStringFunc(body, func(m string) string {
		key := m[2 : len(m)-2]
		if v, ok := vars[key]; ok {
			return v
		}
		missing[key] = true
		return m
	})
	return out, sortedKeys(missing)
}

// sortedKeys returns m's keys in sorted order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type renderFile struct {
	Dest   string
	Source string
	Body   string
	Mode   os.FileMode
	Style  fileStyle
}

func substitutionVars(m *Manifest) map[string]string {
	vars := map[string]string{}
	if m == nil {
		return vars
	}
	for k, v := range m.Vars {
		vars[k] = v
	}
	for k, v := range m.Lists {
		vars[k] = strings.Join(v, ", ")
	}
	return vars
}

func renderAll(m *Manifest) ([]renderFile, error) {
	entries, err := fs.ReadDir(templatesFS, templatesDir)
	if err != nil {
		return nil, fmt.Errorf("internal: reading embedded templates: %w", err)
	}

	docsRoot := docsPathValue(m)
	workflowsDir := docsRoot + "/" + workflowsSubdir
	hooksDir := hooksPathValue(m)

	vars := substitutionVars(m)
	vars[pathsDocsKey] = docsRoot

	var files []renderFile
	var errLines []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		src := templatesDir + "/" + name
		raw, err := fs.ReadFile(templatesFS, src)
		if err != nil {
			return nil, fmt.Errorf("internal: reading embedded %s: %w", src, err)
		}
		body, missing := substitute(string(raw), vars)
		if len(missing) > 0 {
			errLines = append(errLines, fmt.Sprintf("%s: undefined manifest variables: %s", src, strings.Join(missing, ", ")))
			continue
		}
		files = append(files, renderFile{Dest: workflowsDir + "/" + name, Source: src, Body: body})
	}
	if len(errLines) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errLines, "\n"))
	}

	files = append(files, renderFile{Dest: workflowsDir + "/docs-philosophy.md", Source: "docs/docs-philosophy.md", Body: philosophySrc})

	sort.Slice(files, func(i, j int) bool { return files[i].Dest < files[j].Dest })

	files = append(files, buildHookShims(hooksDir)...)

	return files, nil
}

var hookEvents = []string{"pre-commit", "commit-msg", "post-commit"}

func buildHookShims(hooksDir string) []renderFile {
	var files []renderFile
	for _, event := range hookEvents {
		files = append(files, renderFile{
			Dest:   hooksDir + "/" + event,
			Source: "generated",
			Body:   "exec cinch hook " + event + " \"$@\"\n",
			Mode:   0o755,
			Style:  styleShell,
		})
	}
	return files
}

func CmdRender(root string) int {
	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("render", err)
	}

	files, err := renderAll(m)
	if err != nil {
		return output.Fail("render", err)
	}

	for _, f := range files {
		dst := filepath.Join(root, f.Dest)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return output.Fail("render", err)
		}
		content := header(f.Source, f.Body, f.Style) + f.Body
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dst, []byte(content), mode); err != nil {
			return output.Fail("render", err)
		}
		output.Step("rendered %s", f.Dest)
	}
	return 0
}
