package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"cinch/internal/output"
)

const workflowsSubdir = "workflows"

func listWorkflowNames(docsRoot string) ([]string, error) {
	directory := filepath.Join(docsRoot, workflowsSubdir)
	entries, err := os.ReadDir(directory)
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

func workflowsTable(docsRoot string) (string, error) {
	names, err := listWorkflowNames(docsRoot)
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("cinch render has not run — nothing to show")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d %s:\n\n", len(names), output.Plural(len(names), "workflow"))
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

func CmdWorkflows(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("workflows", err)
	}
	table, err := workflowsTable(docsRoot)
	if err != nil {
		return output.Fail("workflows", err)
	}
	fmt.Print(table)
	return 0
}

func CmdWorkflow(root, name string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("workflow", err)
	}
	names, err := listWorkflowNames(docsRoot)
	if err != nil {
		return output.Failf("workflow", "cinch render has not run — nothing to show")
	}
	if !slices.Contains(names, name) {
		return output.Failf("workflow", "%q is not a known workflow — run 'cinch workflows' to see what's available", name)
	}
	data, err := os.ReadFile(filepath.Join(docsRoot, workflowsSubdir, name+".md"))
	if err != nil {
		return output.Fail("workflow", err)
	}
	fmt.Print(string(data))
	return 0
}
