package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	impl "cinch/internal/cinch"
	"cinch/internal/output"
)

var Version = "dev"

const helpText = `cinch — check the operational docs that govern a repository

usage:
  cinch help              print this list; -h and --help are aliases
  cinch version           print the cinch version
  cinch init              scaffold a consumer (fills cinch.yml, renders, activates hooks)
  cinch check [MSGFILE]   run all checks (MSGFILE = in-progress commit message)
  cinch ignores           list every cinch:ignore declaration
  cinch render            render docs/templates and git hook shims from cinch.yml
  cinch hook EVENT [ARGS] git-hook dispatcher the generated shims exec into
  cinch workflows         print the workflow trigger table
  cinch workflow NAME     print one rendered workflow
  cinch index             print every doc's path and title

exit codes: 0 clean, 1 findings, 2 usage error.
`

const bareSynopsis = `cinch — check the operational docs that govern a repository

run 'cinch help' for the full command list.
`

var commands = []string{"help", "version", "init", "check", "ignores", "render", "hook", "workflows", "workflow", "index"}

func main() {
	impl.Version = Version

	if len(os.Args) < 2 {
		os.Exit(bareInvocation())
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case "version":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("version: takes no arguments"))
		}
		fmt.Println(Version)
		os.Exit(0)
	case "init":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("init: takes no arguments"))
		}
		os.Exit(impl.CmdInit("."))
	case "check":
		args := os.Args[2:]
		if len(args) > 1 {
			os.Exit(output.UsageError("check: too many arguments"))
		}
		msgFile := ""
		if len(args) == 1 {
			msgFile = args[0]
			if _, err := os.Stat(msgFile); err != nil {
				os.Exit(output.UsageError("check: cannot read message file: " + msgFile))
			}
		}
		os.Exit(impl.CmdCheck(msgFile))
	case "ignores":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("ignores: takes no arguments"))
		}
		docs, err := impl.ResolveDocsRoot(".")
		if err != nil {
			os.Exit(output.Fail("ignores", err))
		}
		os.Exit(impl.CmdIgnores(docs))
	case "render":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("render: takes no arguments"))
		}
		os.Exit(impl.CmdRender("."))
	case "hook":
		if len(os.Args) < 3 {
			os.Exit(output.UsageError("hook: requires an event argument"))
		}
		os.Exit(impl.CmdHook(".", os.Args[2], os.Args[3:]))
	case "workflows":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("workflows: takes no arguments"))
		}
		os.Exit(impl.CmdWorkflows("."))
	case "workflow":
		if len(os.Args) != 3 {
			os.Exit(output.UsageError("workflow: requires exactly one NAME argument"))
		}
		os.Exit(impl.CmdWorkflow(".", os.Args[2]))
	case "index":
		if len(os.Args) > 2 {
			os.Exit(output.UsageError("index: takes no arguments"))
		}
		os.Exit(impl.CmdIndex("."))
	default:
		os.Exit(unknownCommand(os.Args[1]))
	}
}

func unknownCommand(name string) int {
	msg := fmt.Sprintf("%q is not a command", name)
	if guess, dist := closestCommand(name); guess != "" && dist <= 2 {
		msg += fmt.Sprintf(" (did you mean %q?)", guess)
	}
	return output.UsageError(msg)
}

func bareInvocation() int {
	if impl.ManifestExists(".") {
		fmt.Print(bareSynopsis)
		return 2
	}

	fmt.Println("this doesn't look like an initialized cinch project (no cinch.yml found in the current directory).")

	if impl.IsGitRepo(".") && output.IsInteractiveStdin(os.Stdin) {
		fmt.Print("run 'cinch init' now? [y/N] ")
		if promptYes() {
			return impl.CmdInit(".")
		}
	}

	fmt.Println("run 'cinch init' to get started, or 'cinch help' for the full command list.")
	return 2
}

func promptYes() bool {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

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
