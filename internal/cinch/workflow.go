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
// under docsRoot's workflows subdirectory, without the .md extension —
// sorted.
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
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

// workflowsTable computes the workflow trigger table from whatever is on
// disk right now — nothing persisted, so nothing to go stale. Returns an
// error when there is nothing to show (render hasn't run).
func workflowsTable(docsRoot string) (string, error) {
	names, err := listWorkflowNames(docsRoot)
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("cinch render has not run — nothing to show")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d workflows:\n\n", len(names))
	b.WriteString("| Workflow | Trigger |\n")
	b.WriteString("|---|---|\n")
	for _, name := range names {
		file := name + ".md"
		data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, file))
		if err != nil {
			return "", err
		}
		_, trigger := titleAndTrigger(string(data))
		fmt.Fprintf(&b, "| [%s](%s) | %s |\n", file, file, trigger)
	}
	return b.String(), nil
}

// CmdWorkflows implements `cinch workflows`: prints the computed workflow
// trigger table.
func CmdWorkflows(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return Fail("workflows", err)
	}
	table, err := workflowsTable(docsRoot)
	if err != nil {
		return Fail("workflows", err)
	}
	fmt.Print(table)
	return 0
}

// CmdWorkflow implements `cinch workflow NAME`: prints one rendered
// workflow's full content, so a consumer's AGENTS.md can point agents at a
// command instead of a path that goes stale under a customized paths.docs.
func CmdWorkflow(root, name string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return Fail("workflow", err)
	}
	names, err := listWorkflowNames(docsRoot)
	if err != nil {
		return Failf("workflow", "cinch render has not run — nothing to show")
	}
	if !slices.Contains(names, name) {
		return Failf("workflow", "%q is not a known workflow — run `cinch workflows` to see what's available", name)
	}
	data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, name+".md"))
	if err != nil {
		return Fail("workflow", err)
	}
	fmt.Print(string(data))
	return 0
}
