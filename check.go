package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	indexName = "index.md"
	docsRoot  = ".docs"
)

var linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	root := fs.String("root", docsRoot, "corpus root directory")
	fs.Parse(args)

	corpus := corpusFiles(*root)
	indexPath := filepath.Join(*root, indexName)

	index, err := os.ReadFile(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "C1: missing index: %s\n", indexPath)
		return 1
	}

	entries := indexEntries(index)
	listed := make(map[string]bool, len(entries))
	for _, e := range entries {
		listed[e] = true
	}

	missing := make([]string, 0)
	for _, f := range corpus {
		if !listed[f] {
			missing = append(missing, f)
		}
	}

	phantom := make([]string, 0)
	for _, e := range entries {
		if !existsInCorpus(*root, e) {
			phantom = append(phantom, e)
		}
	}

	if len(missing) == 0 && len(phantom) == 0 {
		return 0
	}

	sort.Strings(missing)
	sort.Strings(phantom)
	for _, f := range missing {
		fmt.Fprintf(os.Stderr, "C1 missing: %s has no entry in %s\n", f, indexName)
	}
	for _, e := range phantom {
		fmt.Fprintf(os.Stderr, "C1 phantom: %s lists %q, which is not a corpus file\n", indexName, e)
	}
	return 1
}

// corpusFiles lists regular files directly in root, excluding the index
// itself: the index is not part of the indexed set.
func corpusFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.Name() == indexName || !e.Type().IsRegular() {
			continue
		}
		files = append(files, e.Name())
	}
	return files
}

// indexEntries extracts the link destinations in index.md, normalized: a
// leading "./" is stripped, empty targets dropped.
func indexEntries(index []byte) []string {
	var entries []string
	for _, m := range linkRe.FindAllSubmatch(index, -1) {
		target := strings.TrimSpace(string(m[1]))
		target = strings.TrimPrefix(target, "./")
		if target == "" {
			continue
		}
		entries = append(entries, target)
	}
	return entries
}

// existsInCorpus reports whether name names a regular file directly in root:
// the corpus is flat, so anything with a path separator or a directory is
// not a corpus file.
func existsInCorpus(root, name string) bool {
	if strings.Contains(name, "/") {
		return false
	}
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.Mode().IsRegular()
}
