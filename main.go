package main

import (
	"fmt"
	"os"

	impl "cinch/internal/cinch"
)

const usage = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

usage:
  cinch init              scaffold a new consumer: manifest (if absent),
                          paths.docs/plans, a full render, activated git
                          hooks (in a git repo), and AGENTS.md (if absent).
                          Idempotent.
  cinch check [MSGFILE]   run all checks against the current directory.
                          MSGFILE, if given, is a path to a file containing
                          the in-progress commit message (wired via a
                          commit-msg git hook) — only the coupling check's
                          rule-reword escape hatch consults it.
  cinch ignores           list every cinch:ignore declaration and its
                          reason. Not a check: always exits 0.
  cinch render            render docs/philosophy.md and docs/templates/*.md into
                          paths.docs's workflows/ subdirectory (default
                          .docs/workflows/), substituting values from
                          cinch.yml, plus git hook shims under
                          paths.hooks (default .githooks/). Idempotent — a
                          re-render diff proves tampering.
  cinch hook EVENT [ARGS] git-hook dispatcher the generated shims exec into
                          (pre-commit, commit-msg). Runs cinch check, then
                          any hooks.EVENT.* entries from cinch.yml whose
                          'when' path prefixes match the staged set.
  cinch workflows         compute and print the workflow trigger table.
  cinch workflow NAME     print one rendered workflow's full content.
  cinch index             compute and print every doc's path and title.

exit codes: 0 clean, 1 findings, 2 usage error.
`

// commands is every subcommand cinch recognizes, in usage order — shared
// between the dispatch switch below and the unknown-command suggestion.
var commands = []string{"init", "check", "ignores", "render", "hook", "workflows", "workflow", "index"}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		if len(os.Args) > 2 {
			os.Exit(impl.UsageErr("init: takes no arguments"))
		}
		os.Exit(impl.CmdInit("."))
	case "check":
		args := os.Args[2:]
		if len(args) > 1 {
			os.Exit(impl.UsageErr("check: too many arguments"))
		}
		msgFile := ""
		if len(args) == 1 {
			msgFile = args[0]
			if _, err := os.Stat(msgFile); err != nil {
				os.Exit(impl.UsageErr("check: cannot read message file: " + msgFile))
			}
		}
		os.Exit(impl.CmdCheck(msgFile))
	case "ignores":
		if len(os.Args) > 2 {
			os.Exit(impl.UsageErr("ignores: takes no arguments"))
		}
		docs, err := impl.ResolveDocsRoot(".")
		if err != nil {
			os.Exit(impl.Fail("ignores", err))
		}
		os.Exit(impl.CmdIgnores(docs))
	case "render":
		if len(os.Args) > 2 {
			os.Exit(impl.UsageErr("render: takes no arguments"))
		}
		os.Exit(impl.CmdRender("."))
	case "hook":
		if len(os.Args) < 3 {
			os.Exit(impl.UsageErr("hook: requires an event argument"))
		}
		os.Exit(impl.CmdHook(".", os.Args[2], os.Args[3:]))
	case "workflows":
		if len(os.Args) > 2 {
			os.Exit(impl.UsageErr("workflows: takes no arguments"))
		}
		os.Exit(impl.CmdWorkflows("."))
	case "workflow":
		if len(os.Args) != 3 {
			os.Exit(impl.UsageErr("workflow: requires exactly one NAME argument"))
		}
		os.Exit(impl.CmdWorkflow(".", os.Args[2]))
	case "index":
		if len(os.Args) > 2 {
			os.Exit(impl.UsageErr("index: takes no arguments"))
		}
		os.Exit(impl.CmdIndex("."))
	default:
		os.Exit(unknownCommand(os.Args[1]))
	}
}

// unknownCommand reports an unrecognized subcommand, suggesting the closest
// known command name when one is close enough to plausibly be a typo.
func unknownCommand(name string) int {
	msg := fmt.Sprintf("cinch: %q is not a command", name)
	if guess, dist := closestCommand(name); guess != "" && dist <= 2 {
		msg += fmt.Sprintf(" (did you mean %q?)", guess)
	}
	msg += " — run 'cinch' for usage."
	fmt.Fprintln(os.Stderr, msg)
	return 1
}

// closestCommand returns the known command nearest to name by edit
// distance, and that distance.
func closestCommand(name string) (string, int) {
	best, bestDist := "", -1
	for _, c := range commands {
		d := levenshtein(name, c)
		if bestDist == -1 || d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist
}

// levenshtein computes the edit distance between a and b.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}
