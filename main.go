package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	impl "cinch/internal/cinch"
	"cinch/internal/output"
)

// Version is the cinch release this binary was built from — "devel" unless
// set at build time via -ldflags "-X main.Version=x.y.z" (make build/install
// do this). No runtime VCS lookup: cinch is typically installed to GOBIN and
// run outside its own repo, where a git-describe would find nothing.
var Version = "devel"

const helpText = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

usage:
  cinch help              print this command list and exit. -h and --help
                          are aliases.
  cinch version           print the cinch version and exit.
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
  cinch move-docs NEW-PATH
                          move the docs root: git mv, rewrite cinch.yml's
                          paths.docs, re-render — one atomic operation.
  cinch hook EVENT [ARGS] git-hook dispatcher the generated shims exec into
                          (pre-commit, commit-msg). Runs cinch check, then
                          any hooks.EVENT.* entries from cinch.yml whose
                          'when' path prefixes match the staged set.
  cinch workflows         compute and print the workflow trigger table.
  cinch workflow NAME     print one rendered workflow's full content.
  cinch index             compute and print every doc's path and title.
  cinch context           compute and print what a session needs to
                          re-orient: branch, plans in flight and their
                          status, the workflow trigger table, staged paths.
                          Not a check — reports state, never a verdict.

exit codes: 0 clean, 1 findings, 2 usage error.
`

// bareSynopsis is shown when cinch is invoked with no subcommand and the
// current directory already has a cinch.yml — a short pointer to the full
// listing (helpText) rather than dumping it unconditionally.
const bareSynopsis = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

run 'cinch help' for the full command list.
`

// commands is every subcommand cinch recognizes, in usage order — shared
// between the dispatch switch below and the unknown-command suggestion.
var commands = []string{"help", "version", "init", "check", "ignores", "render", "move-docs", "hook", "workflows", "workflow", "index", "context"}

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
			os.Exit(output.UsageErr("version: takes no arguments"))
		}
		fmt.Println(Version)
		os.Exit(0)
	case "init":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("init: takes no arguments"))
		}
		os.Exit(impl.CmdInit("."))
	case "check":
		args := os.Args[2:]
		if len(args) > 1 {
			os.Exit(output.UsageErr("check: too many arguments"))
		}
		msgFile := ""
		if len(args) == 1 {
			msgFile = args[0]
			if _, err := os.Stat(msgFile); err != nil {
				os.Exit(output.UsageErr("check: cannot read message file: " + msgFile))
			}
		}
		os.Exit(impl.CmdCheck(msgFile))
	case "ignores":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("ignores: takes no arguments"))
		}
		docs, err := impl.ResolveDocsRoot(".")
		if err != nil {
			os.Exit(output.Fail("ignores", err))
		}
		os.Exit(impl.CmdIgnores(docs))
	case "render":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("render: takes no arguments"))
		}
		os.Exit(impl.CmdRender("."))
	case "move-docs":
		if len(os.Args) != 3 {
			os.Exit(output.UsageErr("move-docs: requires exactly one NEW-PATH argument"))
		}
		os.Exit(impl.CmdMoveDocs(".", os.Args[2]))
	case "hook":
		if len(os.Args) < 3 {
			os.Exit(output.UsageErr("hook: requires an event argument"))
		}
		os.Exit(impl.CmdHook(".", os.Args[2], os.Args[3:]))
	case "workflows":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("workflows: takes no arguments"))
		}
		os.Exit(impl.CmdWorkflows("."))
	case "workflow":
		if len(os.Args) != 3 {
			os.Exit(output.UsageErr("workflow: requires exactly one NAME argument"))
		}
		os.Exit(impl.CmdWorkflow(".", os.Args[2]))
	case "index":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("index: takes no arguments"))
		}
		os.Exit(impl.CmdIndex("."))
	case "context":
		if len(os.Args) > 2 {
			os.Exit(output.UsageErr("context: takes no arguments"))
		}
		os.Exit(impl.CmdContext("."))
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
	msg += " — run 'cinch help' for usage."
	fmt.Fprintln(os.Stderr, output.ColorizeError(msg))
	return 1
}

// bareInvocation handles `cinch` with no arguments: a lighter, state-aware
// response instead of an unconditional full-usage dump. What it says
// depends on whether the current directory looks like an initialized
// cinch project, and — if not — whether it's safe to offer to run
// `cinch init` right now.
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

// promptYes reads one line from stdin and reports whether it's an
// affirmative answer ("y" or "yes", case-insensitive). bareInvocation only
// calls this after confirming stdin is a live terminal, so this never
// blocks indefinitely on input that will never come.
func promptYes() bool {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
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
