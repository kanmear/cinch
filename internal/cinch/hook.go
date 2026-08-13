package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cinch/internal/output"
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
		return output.UsageErr(fmt.Sprintf("hook: unknown event %q", event))
	}
}

func cmdHookPreCommit(root string) int {
	staged, err := gitOutputLines(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	if err != nil {
		return output.Failf("hook", "git diff --cached failed: %s", err.Error())
	}

	ok := CmdCheck("") == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
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
		return output.UsageErr("hook: commit-msg requires a message-file argument")
	}
	msgFile := args[0]

	ok := CmdCheck(msgFile) == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
	}
	// `when` is ignored for commit-msg — a commit message has no changed
	// paths to scope against — so every registered entry always runs.
	if m != nil && !dispatchHooks(root, m, "commit-msg", nil, msgFile) {
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
// args is forwarded to each entry's command (e.g. the commit-msg file path,
// landing in the script's $1) — empty for pre-commit. Every entry runs
// regardless of an earlier one's failure — failures accumulate rather than
// stopping at the first — and the return value is false if any failed.
func dispatchHooks(root string, m *Manifest, event string, staged []string, args ...string) bool {
	ok := true
	for _, name := range m.Names("hooks." + event) {
		command := m.Vars["hooks."+event+"."+name+".run"]
		if command == "" {
			continue
		}
		when := m.List("hooks." + event + "." + name + ".when")
		if !hookWhenMatches(staged, when) {
			output.Skip("hook", event, fmt.Sprintf("%s: no staged path under %v", name, when))
			continue
		}
		if err := runHookCommand(root, command, args...); err != nil {
			output.Failf("hook", "%s: %s failed: %v", event, name, err)
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
// hand-written pre-commit used. args, if given (currently at most one — the
// commit-msg file path), is appended to command as a quoted `$1` reference
// so it reaches the invoked script as its own positional parameter — `sh -c`
// alone doesn't forward the trailing argv to a bare command word unless the
// command text names it. The "hook" placeholder fills sh -c's own $0 slot so
// args starts at $1, mirroring git's own hook contract.
func runHookCommand(root, command string, args ...string) error {
	full := command
	if len(args) == 1 {
		full += ` "$1"`
	}
	cmd := exec.Command("sh", append([]string{"-c", full, "hook"}, args...)...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
