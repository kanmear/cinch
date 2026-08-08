package main

import (
	"fmt"
	"os"
)

const usage = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

usage:
  cinch check [MSGFILE]   run all checks against the current directory.
                          MSGFILE, if given, is a path to a file containing
                          the in-progress commit message (wired via a
                          commit-msg git hook) — only the coupling check's
                          rule-reword escape hatch consults it.
  cinch ignores           list every cinch:ignore declaration and its
                          reason. Not a check: always exits 0.
  cinch render            render philosophy.md and templates/*.md into
                          .agent/workflows/, substituting values from
                          .agent/manifest. Idempotent — a re-render diff
                          proves tampering.

exit codes: 0 clean, 1 findings, 2 usage error.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
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
		os.Exit(cmdCheck(msgFile))
	case "ignores":
		if len(os.Args) > 2 {
			os.Exit(usageError("ignores: takes no arguments"))
		}
		os.Exit(cmdIgnores(docsRoot))
	case "render":
		if len(os.Args) > 2 {
			os.Exit(usageError("render: takes no arguments"))
		}
		os.Exit(cmdRender("."))
	default:
		fmt.Fprintf(os.Stderr, "cinch: %q is not a command\n", os.Args[1])
		os.Exit(1)
	}
}
