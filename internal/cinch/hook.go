package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CmdHook implements `cinch hook <event> [args]`, the dispatcher the
// generated hook shims exec into. The dispatch table (manifest hooks.<event>
// entries) is read at runtime from root's manifest, not baked into the shim
// — so editing the manifest takes effect immediately with no re-render.
func CmdHook(root, event string, args []string) int {
	switch event {
	case "pre-commit":
		return cmdHookPreCommit(root)
	case "commit-msg":
		return cmdHookCommitMsg(root, args)
	default:
		fmt.Fprintf(os.Stderr, "cinch: hook: unknown event %q\n", event)
		return 2
	}
}

func cmdHookPreCommit(root string) int {
	staged, err := gitOutputLines(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: hook: git diff --cached failed: "+err.Error())
		return 1
	}

	ok := CmdCheck("") == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: hook: "+err.Error())
		return 1
	}
	if m != nil && !dispatchHooks(root, m, "pre-commit", staged) {
		ok = false
	}

	if !ok {
		return 1
	}
	return 0
}

func cmdHookCommitMsg(root string, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "cinch: hook: commit-msg requires a message-file argument")
		return 2
	}
	msgFile := args[0]

	ok := CmdCheck(msgFile) == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: hook: "+err.Error())
		return 1
	}
	// `when` is ignored for commit-msg — a commit message has no changed
	// paths to scope against — so every registered entry always runs.
	if m != nil && !dispatchHooks(root, m, "commit-msg", nil) {
		ok = false
	}

	if !ok {
		return 1
	}
	return 0
}

// dispatchHooks runs every hooks.<event>.<name> entry in m, in declaration
// order. staged is the set of staged paths to match each entry's `when`
// list against (path-prefix matching, not globs — a staged path qualifies
// if it starts with one of the `when` values); staged == nil disables `when`
// filtering entirely, so every entry runs regardless (the commit-msg case).
// Every entry runs regardless of an earlier one's failure — failures
// accumulate rather than stopping at the first — and the return value is
// false if any failed.
func dispatchHooks(root string, m *Manifest, event string, staged []string) bool {
	ok := true
	for _, name := range m.Names("hooks." + event) {
		command := m.Vars["hooks."+event+"."+name+".run"]
		if command == "" {
			continue
		}
		when := m.List("hooks." + event + "." + name + ".when")
		if !hookWhenMatches(staged, when) {
			fmt.Fprintf(os.Stderr, "cinch: hook: %s: skipped %s (no staged path under %v)\n", event, name, when)
			continue
		}
		if err := runHookCommand(root, command); err != nil {
			fmt.Fprintf(os.Stderr, "cinch: hook: %s: %s failed: %v\n", event, name, err)
			ok = false
		}
	}
	return ok
}

// hookWhenMatches reports whether a `when` path-prefix list permits an
// entry to run against staged. An empty `when`, or staged == nil (the
// filtering-disabled sentinel), always matches.
func hookWhenMatches(staged, when []string) bool {
	if staged == nil || len(when) == 0 {
		return true
	}
	for _, f := range staged {
		for _, prefix := range when {
			if strings.HasPrefix(f, prefix) {
				return true
			}
		}
	}
	return false
}

// runHookCommand executes command via the shell from root, streaming its
// output straight through — the same `sh -c` + accumulate pattern deltadocs'
// hand-written pre-commit used.
func runHookCommand(root, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
