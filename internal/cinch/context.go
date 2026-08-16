package cinch

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cinch/internal/output"
)

// plansSubdir holds in-flight work artifacts under the docs root. indexList
// deliberately excludes it (transient, not spec); context is the one command
// that wants exactly that excluded set.
const plansSubdir = "plans"

// planStatusPrefix is the line a plan file carries to say where it stands.
// The value is printed verbatim, never parsed into a status vocabulary — a
// vocabulary would be a new convention needing its own enforcement, and this
// command exists to report state, not to mint any.
const planStatusPrefix = "Status:"

// plan is one plan file's decidable facts: where it is, what it's called,
// and what it says about itself.
type plan struct {
	rel    string
	title  string
	status string
}

// listPlans walks docsRoot's plans subdirectory and returns every plan file
// with its title and status line. Recursive, because a consumer may group
// plans into subdirectories (project_deltadocs uses plans/fix/). A missing
// directory returns no plans and no error — a repo with no plans in flight
// is an ordinary state, not a failure.
func listPlans(docsRoot string) []plan {
	plansRoot := filepath.Join(docsRoot, plansSubdir)
	var plans []plan

	filepath.WalkDir(plansRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // plans/ (or a subpath) doesn't exist — nothing to list
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(plansRoot, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		title, _ := titleAndTrigger(string(data))
		if title == "" {
			title = filepath.ToSlash(rel) // a plan with no H1 still exists
		}
		plans = append(plans, plan{
			rel:    filepath.ToSlash(rel),
			title:  title,
			status: planStatus(string(data)),
		})
		return nil
	})

	sort.Slice(plans, func(i, j int) bool { return plans[i].rel < plans[j].rel })
	return plans
}

// planStatus returns the plan's status line verbatim, minus the prefix, or
// "" when it has none. Only the head of the file is considered: the status
// belongs under the title, and a "Status:" appearing deep in prose is not
// one.
func planStatus(body string) string {
	lines := strings.Split(body, "\n")
	if len(lines) > 12 {
		lines = lines[:12]
	}
	for _, l := range lines {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(l), planStatusPrefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// contextReport composes what a session needs to re-orient, from state that
// is decidable right now: the branch, the plans in flight, the workflow
// trigger table, and what is staged. Every section is read from the tree at
// call time — nothing persisted, so nothing to go stale.
//
// This is not a check. It produces no Finding, has no green mark, and never
// reports a red state; a session-start command that silently runs the checks
// would be a sixth check wearing a different hat, and one whose green state
// is "you have read your context" — the proxy metric the principles forbid.
// Sections that cannot be computed announce themselves as skips and the rest
// still prints.
func contextReport(root, docsRoot string) string {
	var b strings.Builder
	git := isGitRepo(root)

	if git {
		branch := "detached HEAD"
		if lines, err := gitOutputLines(root, "branch", "--show-current"); err == nil && len(lines) > 0 {
			branch = lines[0]
		}
		fmt.Fprintf(&b, "on %s\n\n", branch)
	} else {
		output.Skip("context", "branch", "not a git repository")
	}

	plans := listPlans(docsRoot)
	if len(plans) == 0 {
		fmt.Fprintf(&b, "no plans under %s\n", filepath.Join(docsRoot, plansSubdir))
	} else {
		fmt.Fprintf(&b, "%d %s under %s:\n\n", len(plans), pluralize(len(plans), "plan"), filepath.Join(docsRoot, plansSubdir))
		for _, p := range plans {
			fmt.Fprintf(&b, "%s — %s\n", p.rel, p.title)
			if p.status != "" {
				fmt.Fprintf(&b, "    %s %s\n", planStatusPrefix, p.status)
			}
		}
	}

	if table, err := workflowsTable(docsRoot); err == nil {
		b.WriteString("\n")
		b.WriteString(table)
	} else {
		output.Skip("context", "workflows", err.Error())
	}

	if git {
		staged, err := gitOutputLines(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
		switch {
		case err != nil:
			output.Skip("context", "staged", "git diff --cached failed: "+err.Error())
		case len(staged) == 0:
			b.WriteString("\nnothing staged\n")
		default:
			fmt.Fprintf(&b, "\n%d staged %s:\n\n", len(staged), pluralize(len(staged), "path"))
			for _, f := range staged {
				fmt.Fprintf(&b, "%s\n", f)
			}
		}
	} else {
		output.Skip("context", "staged", "not a git repository")
	}

	return b.String()
}

func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// CmdContext implements `cinch context`: prints the session-start report.
// Fails only when the docs root itself can't be resolved — every other
// missing input is a stated skip, since a command meant to be the first
// thing a session runs must not refuse to run because one input is absent.
func CmdContext(root string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("context", err)
	}
	fmt.Print(contextReport(root, docsRoot))
	return 0
}
