package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	if m != nil && !dispatchHooks(root, m, "commit-msg", nil, msgFile) {
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
	files, err := gitOutputLines(root, "diff", "--name-only", "HEAD^", "HEAD")
	if err == nil {
		return files, nil
	}
	return gitOutputLines(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
}

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
