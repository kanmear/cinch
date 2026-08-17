package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// manifestPath is the fixed, non-configurable manifest location. It can't
// itself live under a configurable paths.docs — the manifest is what would
// define that value, so reading it can't depend on it — so it's pinned to
// the repo root instead, decoupled from paths.docs entirely: paths.docs
// controls where rendered/authored docs live, never the manifest itself.
const manifestPath = "cinch.yml"

// Manifest binds the values cinch's workflow templates reference: commands
// and roots (Stage 2 — value substitution only, no shape). The file on disk
// is YAML, but Manifest itself stays a flat `dotted.key -> value` bag —
// nesting in the file is notation for writing dotted keys hierarchically, it
// doesn't grow the manifest a schema.
type Manifest struct {
	Vars map[string]string

	// order is the declaration order of keys as first seen in the manifest
	// file. Hook dispatch (step 7) runs entries in the order the author
	// wrote them, not alphabetically.
	order []string
}

// loadManifest reads root's manifest file. A missing file, a malformed
// document, or a value that fails validate is a named, actionable error —
// render has nothing to substitute without it.
func loadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	m, err := parseManifestFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: not found — cinch render needs a manifest file to bind template variables against, e.g.:\n  paths:\n    docs: .docs", path)
	}
	if err != nil {
		return nil, err
	}
	return m, m.validate(path)
}

// loadManifestOptional reads root's manifest file if one exists, returning
// (nil, nil) when it doesn't — unlike loadManifest, a missing manifest is
// not an error. Callers that must work with zero configuration (check,
// ignores) use this instead. A malformed document, or a value that fails
// validate, is still an error either way.
func loadManifestOptional(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	m, err := parseManifestFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, m.validate(path)
}

// repoLocalPathKeys are the manifest keys naming a directory cinch both
// renders into and resolves independently. render joins them onto the repo
// root (render.go's filepath.Join) while the checks resolve them on their
// own, so a value pointing outside the repository makes render write where
// the checks never look: every check then passes against a corpus none of
// them can see. Keeping these repo-local is what makes the two resolutions
// agree.
var repoLocalPathKeys = []string{pathsDocsKey, pathsHooksKey}

// validate rejects manifest values that would break an invariant cinch
// depends on, reporting against path. Called from both load paths so every
// command fails identically, and never from parseManifestFile — the parser
// stays a generic YAML-to-dotted-key flattener that knows no specific key.
func (m *Manifest) validate(path string) error {
	if m == nil {
		return nil
	}
	for _, key := range repoLocalPathKeys {
		val, ok := m.Vars[key]
		if !ok || val == "" {
			continue // unset falls back to a built-in default, always local
		}
		if !filepath.IsLocal(val) {
			return fmt.Errorf("%s: %s: %q is outside the repository — use a path inside it, e.g. `.docs`. cinch renders into this directory and scans it; a value that escapes makes render write where the checks never look", path, key, val)
		}
	}
	return nil
}

// parseManifestFile reads and parses the manifest at path. Returns the raw
// os.Open error (checkable with os.IsNotExist) when the file is absent.
func parseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // raw, so os.IsNotExist stays checkable
	}
	return parseManifestBytes(data, path)
}

// parseManifestBytes parses manifest content already in memory (e.g. a
// historical revision fetched via `git show`), reporting errors against
// label for consistent messages.
func parseManifestBytes(data []byte, label string) (*Manifest, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	vars := map[string]string{}
	var order []string
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: top-level document must be a mapping", label)
		}
		if err := flattenMapping(label, root, "", vars, &order); err != nil {
			return nil, err
		}
	}

	return &Manifest{Vars: vars, order: order}, nil
}

// flattenMapping walks a YAML mapping node depth-first, flattening nested
// mappings into dotted keys and scalar sequences into comma-joined strings.
// The manifest keeps binding flat string values, not shape — nesting in the
// file is notation only, never a new schema — so this is the one place that
// notation gets collapsed back to the dotted-key model every other consumer
// reads.
func flattenMapping(path string, node *yaml.Node, prefix string, vars map[string]string, order *[]string) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode, valNode := node.Content[i], node.Content[i+1]
		key := prefix + keyNode.Value
		switch valNode.Kind {
		case yaml.MappingNode:
			if err := flattenMapping(path, valNode, key+".", vars, order); err != nil {
				return err
			}
		case yaml.SequenceNode:
			val, err := flattenSequence(path, key, valNode)
			if err != nil {
				return err
			}
			setVar(vars, order, key, val)
		case yaml.ScalarNode:
			setVar(vars, order, key, valNode.Value)
		default:
			return fmt.Errorf("%s:%d: %s: unsupported YAML node", path, valNode.Line, key)
		}
	}
	return nil
}

// flattenSequence joins a scalar list's items with ", " into a single
// string, so List's existing comma-split contract keeps working unchanged
// against a value that came from a YAML list instead of a comma-separated
// scalar.
func flattenSequence(path, key string, node *yaml.Node) (string, error) {
	items := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("%s:%d: %s: only scalar list items are supported", path, item.Line, key)
		}
		items = append(items, item.Value)
	}
	return strings.Join(items, ", "), nil
}

// setVar records key=val, tracking key in order the first time it's seen.
// A key repeated in the file has its value overwritten (last one wins) but
// keeps its original position in order.
func setVar(vars map[string]string, order *[]string, key, val string) {
	if _, seen := vars[key]; !seen {
		*order = append(*order, key)
	}
	vars[key] = val
}

// List returns key's value comma-split and trimmed, dropping empty items.
// A missing key returns nil, distinguishable from a key present but empty
// (which also returns nil, since an empty string has no items to split).
func (m *Manifest) List(key string) []string {
	if m == nil {
		return nil
	}
	val, ok := m.Vars[key]
	if !ok {
		return nil
	}
	var out []string
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Names returns the distinct next path segment after prefix among the
// manifest's keys, in declaration order. Names("hooks.pre-commit") over
//
//	hooks:
//	  pre-commit:
//	    error-codes:
//	      run: scripts/check_error_codes.sh
//	      when: [frontend/]
//	    frontend:
//	      run: make check-frontend
//
// returns ["error-codes", "frontend"].
func (m *Manifest) Names(prefix string) []string {
	if m == nil {
		return nil
	}
	want := prefix + "."
	seen := map[string]bool{}
	var out []string
	for _, key := range m.order {
		if !strings.HasPrefix(key, want) {
			continue
		}
		rest := key[len(want):]
		name, _, _ := strings.Cut(rest, ".")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// isBlankOrComment reports whether line carries no YAML structure worth
// scanning: empty (after trimming) or a `#`-only comment line.
func isBlankOrComment(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}

// lineIndent returns line's leading-space count.
func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// writeManifestValue rewrites a dotted key's value in root's manifest file,
// preserving every other line byte-for-byte — comments, blank lines,
// declaration order, and the original spacing of the target line. Walks the
// key's segments as increasingly-indented YAML mapping keys; the terminal
// segment's scalar is replaced in place.
//
// Deliberately not a YAML round-trip: yaml.v3 retains comment text but
// normalizes layout, which would produce a large spurious diff in a
// hand-commented manifest.
func writeManifestValue(root, key, newVal string) error {
	path := filepath.Join(root, manifestPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(data)
	trailingNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if trailingNewline {
		lines = lines[:len(lines)-1]
	}

	segments := strings.Split(key, ".")
	start, end := 0, len(lines)

	for i, seg := range segments {
		terminal := i == len(segments)-1
		blockIndent := -1
		found := -1

		j := start
		for j < end {
			if isBlankOrComment(lines[j]) {
				j++
				continue
			}
			indent := lineIndent(lines[j])
			if blockIndent == -1 {
				blockIndent = indent
			}
			if indent < blockIndent {
				break // this block ended without a match
			}

			trimmed := strings.TrimSpace(lines[j])
			isMatch := trimmed == seg+":" || strings.HasPrefix(trimmed, seg+": ")
			if isMatch {
				found = j
				break
			}

			// Not our key: skip past this sibling's own nested block (every
			// line more indented than blockIndent) so its descendants are
			// never mistaken for a match at this level.
			j++
			for j < end {
				if isBlankOrComment(lines[j]) {
					j++
					continue
				}
				if lineIndent(lines[j]) <= blockIndent {
					break
				}
				j++
			}
		}

		if found == -1 {
			return fmt.Errorf("%s: %s not found in cinch.yml; add it by hand", path, key)
		}

		if terminal {
			line := lines[found]
			indent := lineIndent(line)
			after := line[indent+len(seg)+1:] // text following "<seg>:"
			comment := ""
			if h := strings.Index(after, "#"); h >= 0 {
				comment = after[h:]
			}
			newLine := strings.Repeat(" ", indent) + seg + ": " + newVal
			if comment != "" {
				newLine += "  " + comment
			}
			lines[found] = newLine

			out := strings.Join(lines, "\n")
			if trailingNewline {
				out += "\n"
			}
			return os.WriteFile(path, []byte(out), 0o644)
		}

		// Descend: the next segment's block is found's children — every
		// following line more indented than found, up to (but excluding) the
		// first line back at found's indent or shallower.
		parentIndent := blockIndent
		start = found + 1
		end = len(lines)
		for k := start; k < len(lines); k++ {
			if isBlankOrComment(lines[k]) {
				continue
			}
			if lineIndent(lines[k]) <= parentIndent {
				end = k
				break
			}
		}
	}

	return fmt.Errorf("%s: %s not found in cinch.yml; add it by hand", path, key)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
