package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// workflowsSubdir is the render output's workflows directory name, shared
// between render.go (where it writes) and here (where it reads).
const workflowsSubdir = "workflows"

// listWorkflowNames returns every rendered workflow's name — its filename
// under docsRoot's workflows subdirectory, without the .md extension,
// excluding the generated index.md — sorted.
func listWorkflowNames(docsRoot string) ([]string, error) {
	dir := filepath.Join(docsRoot, workflowsSubdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if name == "index" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// CmdWorkflows implements `cinch workflows`: prints the generated workflow
// trigger table. Exit 1 if render hasn't run yet — there is nothing to
// print, and that must not read as "this project has zero workflows."
func CmdWorkflows(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}
	data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, "index.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: workflows: cinch render has not run — nothing to show")
		return 1
	}
	fmt.Print(string(data))
	return 0
}

// CmdWorkflow implements `cinch workflow NAME`: prints one rendered
// workflow's full content, so a consumer's AGENTS.md can point agents at a
// command instead of a path that goes stale under a customized paths.docs.
func CmdWorkflow(root, name string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}
	names, err := listWorkflowNames(docsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: workflow: cinch render has not run — nothing to show")
		return 1
	}
	if !slices.Contains(names, name) {
		fmt.Fprintf(os.Stderr, "cinch: workflow: %q is not a known workflow — run `cinch workflows` to see what's available\n", name)
		return 1
	}
	data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, name+".md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}
	fmt.Print(string(data))
	return 0
}
