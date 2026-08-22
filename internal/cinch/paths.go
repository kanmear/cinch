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

func docsPathValue(m *manifest) string {
	return manifestVar(m, pathsDocsKey, defaultDocsPath)
}

func hooksPathValue(m *manifest) string {
	return manifestVar(m, pathsHooksKey, defaultHooksPath)
}

func ResolveDocsRoot(root string) (string, error) {
	m, err := loadManifestOptional(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, docsPathValue(m)), nil
}
