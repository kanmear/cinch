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

func cmdHookPreCommit(root string) int {
	staged, err := stagedPaths(root)
	if err != nil {
		return output.Failf("hook", "git diff --cached failed: %s", err.Error())
	}

	roots, cleanup, err := stagedSnapshot(root)
	defer cleanup()
	if err != nil {
		return output.Fail("hook", err)
	}
	// The rules check and the impact advisory below both scan for markers;
	// one cache makes that a single pass over the tree.
	roots.scans = newMarkerScanCache()
	ok := preCommitChecks(roots) == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
	}

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
		warnIfTreeDiffers(root, m, staged)
		steps := dispatchHooks(root, m, "pre-commit", staged)
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

func cmdHookCommitMsg(root string, args []string) int {
	if len(args) < 1 {
		return output.UsageError("hook: commit-msg requires a message-file argument")
	}
	messageFile := args[0]

	ok := commitMsgChecks(root, messageFile) == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
	}
	if m != nil {
		steps := dispatchHooks(root, m, "commit-msg", nil, messageFile)
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

func cmdHookPostCommit(root string) int {
	committed, err := committedFiles(root)
	if err != nil {
		return output.Failf("hook", "git diff HEAD failed: %s", err.Error())
	}

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
	}
	if m == nil {
		return 0
	}
	steps := dispatchHooks(root, m, "post-commit", committed)
	printHookSummary("post-commit", true, steps)
	if hookStepsFailed(steps) {
		return 1
	}
	return 0
}

func committedFiles(root string) ([]string, error) {
	files, err := gitutil.OutputPaths(root, "diff", "--name-only", "HEAD^", "HEAD")
	if err == nil {
		return files, nil
	}
	return gitutil.OutputPaths(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
}

func stagedPaths(root string) ([]string, error) {
	return gitutil.OutputPaths(root, "diff", "--cached", "--name-only", "--diff-filter=ACMRD")
}

func unstagedPaths(root string) ([]string, error) {
	return gitutil.OutputPaths(root, "diff", "--name-only", "--diff-filter=ACMRD")
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
func dispatchHooks(root string, m *manifest, event string, staged []string, args ...string) []hookStep {
	var steps []hookStep
	for _, script := range hookScripts(m, event) {
		if !hookWhenMatches(staged, script.when) {
			output.Skip(event, fmt.Sprintf("%s: no staged path under %v", script.name, script.when))
			steps = append(steps, hookStep{name: script.name, status: hookSkipped})
			continue
		}
		if err := runHookCommand(root, event, script.command, args...); err != nil {
			output.Failf("hook", "%s: %s failed: %v", event, script.name, err)
			steps = append(steps, hookStep{name: script.name, status: hookFailed, err: err})
			continue
		}
		steps = append(steps, hookStep{name: script.name, status: hookPassed})
	}
	return steps
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
		good         bool
	}
	var rows []row
	ran, failed := 0, 0
	if event != "post-commit" {
		result := "ok"
		if !checksOK {
			result = "failed"
		}
		rows = append(rows, row{"checks", result, checksOK})
		ran++
		if !checksOK {
			failed++
		}
	}
	scriptsRan := false
	for _, step := range steps {
		switch step.status {
		case hookPassed:
			rows = append(rows, row{step.name, "ok", true})
			ran++
			scriptsRan = true
		case hookFailed:
			rows = append(rows, row{step.name, fmt.Sprintf("failed (%v)", step.err), false})
			ran++
			failed++
			scriptsRan = true
		case hookSkipped:
			rows = append(rows, row{step.name, "skipped", true})
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
	output.Note("hook", "%s: %s", event, output.StatusText(failed == 0, headline))
	for _, r := range rows {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, r.name, output.StatusText(r.good, r.result))
	}
}

// warnIfTreeDiffers says so when a pre-commit script, which reads the
// working tree, would see something other than what is being committed.
func warnIfTreeDiffers(root string, m *manifest, staged []string) {
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
	unstaged, err := unstagedPaths(root)
	if err != nil {
		return
	}
	untracked, err := gitutil.OutputPaths(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return
	}
	if total := len(unstaged) + len(untracked); total > 0 {
		output.Warn("hook", "pre-commit scripts run in the working tree, which differs from this commit in %d %s (%d unstaged, %d untracked)",
			total, output.Plural(total, "file"), len(unstaged), len(untracked))
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
