package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestPath is the fixed, non-configurable manifest location — no config
// key for it, matching docsRoot's convention in check.go.
const manifestPath = ".agent/manifest"

// Manifest binds the values cinch's workflow templates reference: commands
// and roots (Stage 2 — value substitution only, no shape). It's a flat
// `dotted.key = value` text format with a hand-written parser — no
// dependency, no schema.
type Manifest struct {
	Vars map[string]string
}

// loadManifest reads root's manifest file. A missing file, or a malformed
// line, is a named, actionable error — render has nothing to substitute
// without it.
func loadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, manifestPath)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: not found — cinch render needs a manifest binding its template variables, e.g.:\n  paths.domain = .agent/domain", path)
	}
	defer f.Close()

	vars := map[string]string{}
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Text()
		text := strings.TrimSpace(raw)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, val, ok := strings.Cut(text, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: not a `key = value` line: %q", path, line, raw)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, line)
		}
		vars[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Manifest{Vars: vars}, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
