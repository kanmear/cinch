// Package manifest loads and flattens a consumer's manifest.yml, and resolves
// the paths the other packages share (the .agent directory, sorted key
// iteration).
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest is the loaded manifest.yml plus its flattened variable view.
type Manifest struct {
	Raw  map[string]any
	Vars map[string]string // dotted keys -> literal values
}

// AgentDir returns the absolute path of the .agent directory.
func AgentDir(root string) string { return filepath.Join(root, ".agent") }

func LoadManifest(root string) (*Manifest, error) {
	path := filepath.Join(AgentDir(root), "manifest.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest.yml: %w", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parsing manifest.yml: %w", err)
	}

	vars := map[string]string{}
	flatten("", raw, vars)

	// Alias: templates say {{commands.check}}; the manifest stores it under
	// development.commands.check.
	for k, v := range vars {
		if rest, ok := strings.CutPrefix(k, "development."); ok {
			if _, taken := vars[rest]; !taken {
				vars[rest] = v
			}
		}
	}

	return &Manifest{Raw: raw, Vars: vars}, nil
}

// flatten walks a decoded YAML tree into dotted scalar keys. Sequences of
// scalars are joined with ", "; sequences of mappings are indexed by any "id"
// field they carry, falling back to their position.
func flatten(prefix string, node any, out map[string]string) {
	join := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + "." + k
	}

	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			flatten(join(k), child, out)
		}
	case []any:
		scalars := make([]string, 0, len(v))
		allScalar := true
		for i, child := range v {
			if m, ok := child.(map[string]any); ok {
				allScalar = false
				key := fmt.Sprintf("%d", i)
				if id, ok := m["id"].(string); ok {
					key = id
				}
				flatten(join(key), child, out)
				continue
			}
			scalars = append(scalars, scalar(child))
		}
		if allScalar {
			out[prefix] = strings.Join(scalars, ", ")
		}
	default:
		if prefix != "" {
			out[prefix] = scalar(v)
		}
	}
}

func scalar(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// PathValues returns every dotted key under "paths." with its value.
func (m *Manifest) PathValues() map[string]string {
	out := map[string]string{}
	for k, v := range m.Vars {
		if strings.HasPrefix(k, "paths.") {
			out[k] = v
		}
	}
	return out
}

// TestTierCommands returns tier id -> declared cmd id.
func (m *Manifest) TestTierCommands() map[string]string {
	out := map[string]string{}
	tax, _ := m.Raw["taxonomy"].(map[string]any)
	tiers, _ := tax["test_tiers"].([]any)
	for _, t := range tiers {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		id, _ := tm["id"].(string)
		cmd, ok := tm["cmd"].(string)
		if id != "" && ok {
			out[id] = cmd
		}
	}
	return out
}

// CommandKeys returns the set of keys under development.commands.
func (m *Manifest) CommandKeys() map[string]bool {
	out := map[string]bool{}
	dev, _ := m.Raw["development"].(map[string]any)
	cmds, _ := dev["commands"].(map[string]any)
	for k := range cmds {
		out[k] = true
	}
	return out
}

// HasVars reports whether every key exists in the flattened variable view —
// key presence is a shape declaration (D063).
func (m *Manifest) HasVars(keys []string) bool {
	for _, k := range keys {
		if _, ok := m.Vars[k]; !ok {
			return false
		}
	}
	return true
}

// SortedKeys returns the keys of m in sorted order — the one iteration order
// the CLI prints and compares (deterministic output, D029).
func SortedKeys[T any](m map[string]T) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
