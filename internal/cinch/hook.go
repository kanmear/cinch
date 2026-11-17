package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/output"
)

// hookEnvironmentVariable tells a hook script which event it runs under.
const hookEnvironmentVariable = "CINCH_HOOK"

type hookStatus int

const (
	hookPassed hookStatus = iota
	hookFailed
	hookSkipped
)

// hookStep is one project script's outcome at one hook event.
type hookStep struct {
	name   string
	status hookStatus
	err    error
}

type hookScript struct {
	name    string
	command string
	when    []string
}

func CmdHook(root, event string, args []string) int {
	switch event {
	case "pre-commit":
		return cmdHookPreCommit(root)
	case "commit-msg":
		return cmdHookCommitMsg(root, args)
	case "post-commit":
		return cmdHookPostCommit(root)
	default:
		return output.UsageError(fmt.Sprintf("hook: unknown event %q", event))
	}
}

func cmdHookPreCommit(repoRoot string) int {
	staged, err := stagedPaths(repoRoot)
	if err != nil {
		return output.Failf("hook", "git diff --cached failed: %s", err.Error())
	}

	// Loaded before the checks so a malformed cinch.yml is reported once, here:
	// the checks would fail on it too, and say the same thing a second time.
	m, err := loadManifestOptional(repoRoot)
	if err != nil {
		return output.Fail("hook", err)
	}

	roots, cleanup, err := stagedSnapshot(repoRoot)
	defer cleanup()
	if err != nil {
		return output.Fail("hook", err)
	}
	// The rules check and the impact advisory below both scan for markers;
	// one cache makes that a single pass over the tree.
	roots.scans = newMarkerScanCache()
	ok := preCommitChecks(roots) == 0

	// The checks and the impact advisory read the same index snapshot, so
	// the advisory describes what is being committed. Only dispatchHooks
	// (project scripts) sees the working tree: they read whatever is on disk.
	if docsRoot, dErr := ResolveDocsRoot(roots.fsRoot); dErr == nil {
		if hits, iErr := buildImpact(roots, docsRoot, staged, m.list(pathsExcludeKey)); iErr == nil {
			printImpact(hits, true)
		}
		// iErr is deliberately swallowed: this advisory is a bonus nobody
		// asked for on this path, and ok must never move because of it —
		// printing a failure-styled message from a path that still exits 0
		// would be actively misleading.
	}

	if m != nil {
		warnIfTreeDiffers(repoRoot, m, staged)
		steps := dispatchHooks(repoRoot, m, "pre-commit", staged)
		printHookSummary("pre-commit", ok, steps)
		if hookStepsFailed(steps) {
			ok = false
		}
	}

	if !ok {
		return 1
	}
	return 0
}

func cmdHookCommitMsg(repoRoot string, args []string) int {
	if len(args) < 1 {
		return output.UsageError("hook: commit-msg requires a message-file argument")
	}
	messageFile := args[0]

	// Loaded before the checks for the same reason as in cmdHookPreCommit.
	m, err := loadManifestOptional(repoRoot)
	if err != nil {
		return output.Fail("hook", err)
	}

	ok := commitMsgChecks(repoRoot, messageFile) == 0

	if m != nil {
		steps := dispatchHooks(repoRoot, m, "commit-msg", nil, messageFile)
		printHookSummary("commit-msg", ok, steps)
		if hookStepsFailed(steps) {
			ok = false
		}
	}

	if !ok {
		return 1
	}
	return 0
}

func cmdHookPostCommit(repoRoot string) int {
	committed, err := committedFiles(repoRoot)
	if err != nil {
		return output.Failf("hook", "git diff HEAD failed: %s", err.Error())
	}

	m, err := loadManifestOptional(repoRoot)
	if err != nil {
		return output.Fail("hook", err)
	}
	if m == nil {
		return 0
	}
	steps := dispatchHooks(repoRoot, m, "post-commit", committed)
	printHookSummary("post-commit", true, steps)
	if hookStepsFailed(steps) {
		return 1
	}
	return 0
}

func committedFiles(repoRoot string) ([]string, error) {
	files, err := gitutil.OutputPaths(repoRoot, "diff", "--name-only", "HEAD^", "HEAD")
	if err == nil {
		return files, nil
	}
	return gitutil.OutputPaths(repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
}

func stagedPaths(repoRoot string) ([]string, error) {
	return gitutil.OutputPaths(repoRoot, "diff", "--cached", "--name-only", "--diff-filter=ACMRD")
}

func unstagedPaths(repoRoot string) ([]string, error) {
	return gitutil.OutputPaths(repoRoot, "diff", "--name-only", "--diff-filter=ACMRD")
}

// hookScripts lists the scripts registered under hooks.<event>, in cinch.yml
// order, leaving out any without a run command.
func hookScripts(m *manifest, event string) []hookScript {
	var scripts []hookScript
	for _, name := range m.names("hooks." + event) {
		command := m.vars["hooks."+event+"."+name+".run"]
		if command == "" {
			continue
		}
		scripts = append(scripts, hookScript{name, command, m.list("hooks." + event + "." + name + ".when")})
	}
	return scripts
}

// dispatchHooks runs every script that passes its when filter and keeps going
// after a failure, so a later step (a logger, say) still runs; the caller
// prints the summary that makes the failure impossible to miss.
func dispatchHooks(repoRoot string, m *manifest, event string, staged []string, args ...string) []hookStep {
	var steps []hookStep
	for _, script := range hookScripts(m, event) {
		if !hookWhenMatches(staged, script.when) {
			output.Skip(event, fmt.Sprintf("%s: no staged path under %v", script.name, script.when))
			steps = append(steps, hookStep{name: script.name, status: hookSkipped})
			continue
		}
		// Script output is bare, so this line is what tells it from cinch's own.
		fmt.Fprintln(os.Stderr, output.Dim(fmt.Sprintf("%s: %s: %s", event, script.name, firstLine(script.command))))
		// A failure isn't reported here: the summary row names it and the
		// error, and is the last thing printed.
		if err := runHookCommand(repoRoot, event, script.command, args...); err != nil {
			steps = append(steps, hookStep{name: script.name, status: hookFailed, err: err})
			continue
		}
		steps = append(steps, hookStep{name: script.name, status: hookPassed})
	}
	return steps
}

// firstLine shortens a multi-line script to its first line plus an ellipsis.
func firstLine(command string) string {
	first, rest, multiline := strings.Cut(strings.TrimSpace(command), "\n")
	if multiline && strings.TrimSpace(rest) != "" {
		return strings.TrimSpace(first) + " ..."
	}
	return strings.TrimSpace(first)
}

func hookStepsFailed(steps []hookStep) bool {
	for _, step := range steps {
		if step.status == hookFailed {
			return true
		}
	}
	return false
}

// printHookSummary ends a hook's output with one row per step, so a failure
// printed above a long script's output is still the last thing read. Nothing
// is printed when no project script ran: the checks report themselves.
// post-commit has no built-in checks, hence no row for them.
func printHookSummary(event string, checksOK bool, steps []hookStep) {
	type row struct {
		name, result string
		color        func(string) string
	}
	good := func(ok bool) func(string) string {
		return func(s string) string { return output.StatusText(ok, s) }
	}
	var rows []row
	ran, failed := 0, 0
	if event != "post-commit" {
		result := "ok"
		if !checksOK {
			result = "failed"
		}
		rows = append(rows, row{"checks", result, good(checksOK)})
		ran++
		if !checksOK {
			failed++
		}
	}
	scriptsRan := false
	for _, step := range steps {
		switch step.status {
		case hookPassed:
			rows = append(rows, row{step.name, "ok", good(true)})
			ran++
			scriptsRan = true
		case hookFailed:
			rows = append(rows, row{step.name, fmt.Sprintf("failed (%v)", step.err), good(false)})
			ran++
			failed++
			scriptsRan = true
		case hookSkipped:
			rows = append(rows, row{step.name, "skipped", output.SkipText})
		}
	}
	if !scriptsRan {
		return
	}

	width := 0
	for _, r := range rows {
		width = max(width, len(r.name))
	}
	headline := fmt.Sprintf("%d %s ok", ran, output.Plural(ran, "step"))
	if failed > 0 {
		headline = fmt.Sprintf("%d of %d %s failed", failed, ran, output.Plural(ran, "step"))
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", event, output.StatusText(failed == 0, headline))
	for _, r := range rows {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, r.name, r.color(r.result))
	}
}

// warnIfTreeDiffers says so when a pre-commit script, which reads the
// working tree, would see something other than what is being committed.
func warnIfTreeDiffers(repoRoot string, m *manifest, staged []string) {
	runs := false
	for _, script := range hookScripts(m, "pre-commit") {
		if hookWhenMatches(staged, script.when) {
			runs = true
			break
		}
	}
	if !runs {
		return
	}
	unstaged, err := unstagedPaths(repoRoot)
	if err != nil {
		return
	}
	untracked, err := gitutil.OutputPaths(repoRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return
	}
	if total := len(unstaged) + len(untracked); total > 0 {
		verb := "differ"
		if total == 1 {
			verb = "differs"
		}
		output.Warn("hook", "pre-commit scripts run in the working tree, not this commit\n  %d %s %s: %d unstaged, %d untracked",
			total, output.Plural(total, "file"), verb, len(unstaged), len(untracked))
	}
}

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

func runHookCommand(root, event, command string, args ...string) error {
	full := command
	if len(args) == 1 {
		full += ` "$1"`
	}
	cmd := exec.Command("sh", append([]string{"-c", full, "hook"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), hookEnvironmentVariable+"="+event)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
