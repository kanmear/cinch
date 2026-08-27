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

	"gopkg.in/yaml.v3"
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

	if err := writeRenderedFiles(root, files); err != nil {
		return output.Fail("render", err)
	}

	if err := syncRequireCinch(root); err != nil {
		return output.Fail("render", err)
	}

	return 0
}

func writeRenderedFiles(root string, files []renderFile) error {
	for _, f := range files {
		destination := filepath.Join(root, f.destination)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		content := header(f.source, f.body, f.style) + f.body
		mode := f.mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(destination, []byte(content), mode); err != nil {
			return err
		}
		output.Step("rendered %s", f.destination)
	}
	return nil
}

// syncRequireCinch keeps checkPin's "reinstall and re-run cinch render"
// remediation true: it updates an exact require.cinch pin to match the
// installed version, leaving unset pins, ">=" ranges, and dev builds alone.
func syncRequireCinch(root string) error {
	if Version == "dev" {
		return nil
	}

	path := filepath.Join(root, manifestPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	node := findRequireCinchNode(&doc)
	if node == nil {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(node.Value), ">=") {
		return nil
	}
	if node.Value == Version {
		return nil
	}

	old := node.Value
	patched, err := patchScalarLine(string(data), node, Version)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		return err
	}
	output.Step("synced require.cinch: %s -> %s", old, Version)
	return nil
}

func patchScalarLine(content string, node *yaml.Node, newValue string) (string, error) {
	lines := strings.SplitAfter(content, "\n")
	idx := node.Line - 1
	if idx < 0 || idx >= len(lines) {
		return "", fmt.Errorf("line %d: out of range", node.Line)
	}
	line := lines[idx]
	col := node.Column - 1
	if col < 0 || col > len(line) {
		return "", fmt.Errorf("line %d: column %d out of range", node.Line, node.Column)
	}
	prefix, rest := line[:col], line[col:]

	var end int
	switch {
	case node.Style&yaml.SingleQuotedStyle != 0:
		end = scanQuotedScalar(rest, '\'')
	case node.Style&yaml.DoubleQuotedStyle != 0:
		end = scanQuotedScalar(rest, '"')
	default:
		end = scanPlainScalar(rest)
	}
	if end < 0 {
		return "", fmt.Errorf("line %d: could not locate scalar value", node.Line)
	}

	lines[idx] = prefix + formatScalar(newValue, node.Style) + rest[end:]
	return strings.Join(lines, ""), nil
}

// s[0] must be the opening quote. A doubled quote (two apostrophes) is
// YAML's escape for a literal quote inside a single-quoted scalar, not the
// closing delimiter.
func scanQuotedScalar(s string, quote byte) int {
	for i := 1; i < len(s); i++ {
		if s[i] != quote {
			continue
		}
		if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
			i++
			continue
		}
		return i + 1
	}
	return -1
}

func scanPlainScalar(s string) int {
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '#' && s[i] != '\r' && s[i] != '\n' {
		i++
	}
	return i
}

func formatScalar(value string, style yaml.Style) string {
	switch {
	case style&yaml.SingleQuotedStyle != 0:
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	case style&yaml.DoubleQuotedStyle != 0:
		return `"` + value + `"`
	default:
		return value
	}
}

func findRequireCinchNode(doc *yaml.Node) *yaml.Node {
	if len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "require" {
			continue
		}
		requireNode := root.Content[i+1]
		if requireNode.Kind != yaml.MappingNode {
			return nil
		}
		for j := 0; j+1 < len(requireNode.Content); j += 2 {
			if requireNode.Content[j].Value == "cinch" {
				return requireNode.Content[j+1]
			}
		}
		return nil
	}
	return nil
}
