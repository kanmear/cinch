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

// loadManifest reads root's manifest file. A missing file, or a malformed
// document, is a named, actionable error — render has nothing to substitute
// without it.
func loadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	m, err := parseManifestFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: not found — cinch render needs a manifest file to bind template variables against, e.g.:\n  paths:\n    docs: .docs", path)
	}
	return m, err
}

// loadManifestOptional reads root's manifest file if one exists, returning
// (nil, nil) when it doesn't — unlike loadManifest, a missing manifest is
// not an error. Callers that must work with zero configuration (check,
// ignores) use this instead. A malformed document is still an error either
// way.
func loadManifestOptional(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	m, err := parseManifestFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return m, err
}

// parseManifestFile reads and parses the manifest at path. Returns the raw
// os.Open error (checkable with os.IsNotExist) when the file is absent.
func parseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	vars := map[string]string{}
	var order []string
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: top-level document must be a mapping", path)
		}
		if err := flattenMapping(path, root, "", vars, &order); err != nil {
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

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
