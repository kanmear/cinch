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
                          cinch_manifest, plus git hook shims under
                          paths.hooks (default .githooks/). Idempotent — a
                          re-render diff proves tampering.
  cinch hook EVENT [ARGS] git-hook dispatcher the generated shims exec into
                          (pre-commit, commit-msg). Runs cinch check, then
                          any hooks.EVENT.* entries from cinch_manifest whose
                          'when' path prefixes match the staged set.
  cinch workflows         compute and print the workflow trigger table.
  cinch workflow NAME     print one rendered workflow's full content.
  cinch docs              compute and print every doc's path and title.

exit codes: 0 clean, 1 findings, 2 usage error.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		if len(os.Args) > 2 {
			os.Exit(usageError("init: takes no arguments"))
		}
		os.Exit(impl.CmdInit("."))
	case "check":
		args := os.Args[2:]
		if len(args) > 1 {
			os.Exit(usageError("check: too many arguments"))
		}
		msgFile := ""
		if len(args) == 1 {
			msgFile = args[0]
			if _, err := os.Stat(msgFile); err != nil {
				os.Exit(usageError("check: cannot read message file: " + msgFile))
			}
		}
		os.Exit(impl.CmdCheck(msgFile))
	case "ignores":
		if len(os.Args) > 2 {
			os.Exit(usageError("ignores: takes no arguments"))
		}
		docs, err := impl.ResolveDocsRoot(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
			os.Exit(1)
		}
		os.Exit(impl.CmdIgnores(docs))
	case "render":
		if len(os.Args) > 2 {
			os.Exit(usageError("render: takes no arguments"))
		}
		os.Exit(impl.CmdRender("."))
	case "hook":
		if len(os.Args) < 3 {
			os.Exit(usageError("hook: requires an event argument"))
		}
		os.Exit(impl.CmdHook(".", os.Args[2], os.Args[3:]))
	case "workflows":
		if len(os.Args) > 2 {
			os.Exit(usageError("workflows: takes no arguments"))
		}
		os.Exit(impl.CmdWorkflows("."))
	case "workflow":
		if len(os.Args) != 3 {
			os.Exit(usageError("workflow: requires exactly one NAME argument"))
		}
		os.Exit(impl.CmdWorkflow(".", os.Args[2]))
	case "docs":
		if len(os.Args) > 2 {
			os.Exit(usageError("docs: takes no arguments"))
		}
		os.Exit(impl.CmdDocs("."))
	default:
		fmt.Fprintf(os.Stderr, "cinch: %q is not a command\n", os.Args[1])
		os.Exit(1)
	}
}

func usageError(msg string) int {
	fmt.Fprintln(os.Stderr, "cinch: "+msg)
	return 2
}
