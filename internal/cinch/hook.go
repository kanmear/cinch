package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/output"
)

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
			printImpact(hits)
		}
		// iErr is deliberately swallowed: this advisory is a bonus nobody
		// asked for on this path, and ok must never move because of it —
		// printing a failure-styled message from a path that still exits 0
		// would be actively misleading.
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
		return output.UsageError("hook: commit-msg requires a message-file argument")
	}
	messageFile := args[0]

	ok := commitMsgChecks(root, messageFile) == 0

	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("hook", err)
	}
	if m != nil && !dispatchHooks(root, m, "commit-msg", nil, messageFile) {
		ok = false
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
	ok := m == nil || dispatchHooks(root, m, "post-commit", committed)

	if !ok {
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

func dispatchHooks(root string, m *manifest, event string, staged []string, args ...string) bool {
	ok := true
	for _, name := range m.names("hooks." + event) {
		command := m.vars["hooks."+event+"."+name+".run"]
		if command == "" {
			continue
		}
		when := m.list("hooks." + event + "." + name + ".when")
		if !hookWhenMatches(staged, when) {
			output.Skip(event, fmt.Sprintf("%s: no staged path under %v", name, when))
			continue
		}
		if err := runHookCommand(root, command, args...); err != nil {
			output.Failf("hook", "%s: %s failed: %v", event, name, err)
			ok = false
		}
	}
	return ok
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
