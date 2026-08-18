package cinch

import (
	"path/filepath"
)

const (
	pathsDocsKey     = "paths.docs"
	defaultDocsPath  = ".docs"
	pathsHooksKey    = "paths.hooks"
	defaultHooksPath = ".githooks"
)

func docsPathValue(m *Manifest) string {
	return manifestVar(m, pathsDocsKey, defaultDocsPath)
}

func hooksPathValue(m *Manifest) string {
	return manifestVar(m, pathsHooksKey, defaultHooksPath)
}

func ResolveDocsRoot(root string) (string, error) {
	m, err := loadManifestOptional(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, docsPathValue(m)), nil
}

// relTo returns path relative to base (best-effort: git's own output is
// always repo-relative, but filesystem walks may produce absolute paths).
func relTo(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}
