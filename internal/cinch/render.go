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

const templatesDirectory = "docs/templates"

var varRe = regexp.MustCompile(`\{\{([a-zA-Z0-9_.-]+)\}\}`)

var hookEvents = []string{"pre-commit", "commit-msg", "post-commit"}

type renderFile struct {
	source      string
	destination string
	body        string
	mode        os.FileMode
	style       fileStyle
}

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

func substitutionVars(m *manifest) map[string]string {
	vars := map[string]string{}
	if m == nil {
		return vars
	}
	for k, v := range m.vars {
		vars[k] = v
	}
	for k, v := range m.lists {
		vars[k] = strings.Join(v, ", ")
	}
	return vars
}

func renderAll(m *manifest) ([]renderFile, error) {
	entries, err := fs.ReadDir(templatesFS, templatesDirectory)
	if err != nil {
		return nil, fmt.Errorf("internal: reading embedded templates: %w", err)
	}

	docsRoot := docsPathValue(m)
	workflowsDirectory := docsRoot + "/" + workflowsSubdir
	hooksDirectory := hooksPathValue(m)

	vars := substitutionVars(m)
	vars[pathsDocsKey] = docsRoot

	var files []renderFile
	var errorLines []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		source := templatesDirectory + "/" + name
		raw, err := fs.ReadFile(templatesFS, source)
		if err != nil {
			return nil, fmt.Errorf("internal: reading embedded %s: %w", source, err)
		}
		body, missing := substitute(string(raw), vars)
		if len(missing) > 0 {
			errorLines = append(errorLines, fmt.Sprintf("%s: undefined manifest variables: %s", source, strings.Join(missing, ", ")))
			continue
		}
		files = append(files, renderFile{source: source, destination: workflowsDirectory + "/" + name, body: body})
	}
	if len(errorLines) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errorLines, "\n"))
	}

	files = append(files, renderFile{source: "docs/docs-philosophy.md", destination: workflowsDirectory + "/docs-philosophy.md", body: philosophySrc})

	sort.Slice(files, func(i, j int) bool { return files[i].destination < files[j].destination })

	files = append(files, buildHookShims(hooksDirectory)...)

	return files, nil
}

func buildHookShims(hooksDirectory string) []renderFile {
	var files []renderFile
	for _, event := range hookEvents {
		files = append(files, renderFile{
			source:      "generated",
			destination: hooksDirectory + "/" + event,
			body:        "exec cinch hook " + event + " \"$@\"\n",
			mode:        0o755,
			style:       styleShell,
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
		destination := filepath.Join(root, f.destination)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return output.Fail("render", err)
		}
		content := header(f.source, f.body, f.style) + f.body
		mode := f.mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(destination, []byte(content), mode); err != nil {
			return output.Fail("render", err)
		}
		output.Step("rendered %s", f.destination)
	}
	return 0
}
