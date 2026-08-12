package cinch

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// indexList walks docsRoot right now and returns every doc's path and title,
// one line each — the on-demand replacement for a persisted doc map.
// Nothing is written, so nothing can drift from disk; a re-run always
// reflects the corpus as it currently stands.
//
// Excludes plans/ (transient work artifacts). A doc with no `# Title` line
// is skipped rather than erroring — this is a convenience listing, not a
// validated build step the way `cinch render` is. Returns an error when
// there is nothing to show.
func indexList(docsRoot string) (string, error) {
	type entry struct {
		rel   string
		title string
	}
	var entries []entry

	walkErr := filepath.WalkDir(docsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // docsRoot (or a subpath) doesn't exist — nothing to list
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(docsRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "plans/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		title, _ := titleAndTrigger(string(data))
		if title == "" {
			return nil
		}
		entries = append(entries, entry{rel: rel, title: title})
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("cinch render has not run — nothing to show")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s — %s\n", e.rel, e.title)
	}
	return b.String(), nil
}

// CmdIndex implements `cinch index`: prints the computed doc list.
func CmdIndex(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}
	list, err := indexList(docsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: index: "+err.Error())
		return 1
	}
	fmt.Print(list)
	return 0
}
