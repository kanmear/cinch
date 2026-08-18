package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestPath = "cinch.yml"

type Manifest struct {
	Vars  map[string]string
	Lists map[string][]string
	order []string
}

func loadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	m, err := parseManifestFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: not found (run 'cinch init')", path)
	}
	if err != nil {
		return nil, err
	}
	return m, m.validate(path)
}

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

func ManifestExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, manifestPath))
	return err == nil
}

// manifestVar returns the value of key in m, or def if unset or empty.
func manifestVar(m *Manifest, key, def string) string {
	if m != nil {
		if v, ok := m.Vars[key]; ok && v != "" {
			return v
		}
	}
	return def
}

// manifestSetting returns the non-empty value of key from the optional
// manifest at root. ok is false when the manifest is absent or the key is
// unset; err is non-nil only when the manifest exists but is malformed.
func manifestSetting(root, key string) (val string, ok bool, err error) {
	m, err := loadManifestOptional(root)
	if err != nil || m == nil {
		return "", false, err
	}
	v, present := m.Vars[key]
	if !present || v == "" {
		return "", false, nil
	}
	return v, true, nil
}

var repoLocalPathKeys = []string{pathsDocsKey, pathsHooksKey}

func (m *Manifest) validate(path string) error {
	if m == nil {
		return nil
	}
	for _, key := range repoLocalPathKeys {
		val, ok := m.Vars[key]
		if !ok || val == "" {
			continue
		}
		if !filepath.IsLocal(val) {
			return fmt.Errorf("%s: %s: %q must be inside the repository", path, key, val)
		}
	}
	return nil
}

func parseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseManifestBytes(data, path)
}

func parseManifestBytes(data []byte, label string) (*Manifest, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	vars := map[string]string{}
	lists := map[string][]string{}
	var order []string
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: top-level document must be a mapping", label)
		}
		if err := flattenMapping(label, root, "", vars, lists, &order); err != nil {
			return nil, err
		}
	}

	return &Manifest{Vars: vars, Lists: lists, order: order}, nil
}

func flattenMapping(path string, node *yaml.Node, prefix string, vars map[string]string, lists map[string][]string, order *[]string) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode, valNode := node.Content[i], node.Content[i+1]
		key := prefix + keyNode.Value
		switch valNode.Kind {
		case yaml.MappingNode:
			if err := flattenMapping(path, valNode, key+".", vars, lists, order); err != nil {
				return err
			}
		case yaml.SequenceNode:
			items, err := flattenSequence(path, key, valNode)
			if err != nil {
				return err
			}
			if _, seen := lists[key]; !seen {
				*order = append(*order, key)
			}
			lists[key] = items
		case yaml.ScalarNode:
			setVar(vars, order, key, valNode.Value)
		default:
			return fmt.Errorf("%s:%d: %s: unsupported YAML node", path, valNode.Line, key)
		}
	}
	return nil
}

func flattenSequence(path, key string, node *yaml.Node) ([]string, error) {
	items := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s:%d: %s: only scalar list items are supported", path, item.Line, key)
		}
		items = append(items, item.Value)
	}
	return items, nil
}

func setVar(vars map[string]string, order *[]string, key, val string) {
	if _, seen := vars[key]; !seen {
		*order = append(*order, key)
	}
	vars[key] = val
}

func (m *Manifest) List(key string) []string {
	if m == nil {
		return nil
	}
	items, ok := m.Lists[key]
	if !ok {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}

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
