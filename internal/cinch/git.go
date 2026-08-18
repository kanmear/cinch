package cinch

import (
	"bufio"
	"os/exec"
	"strings"
)

// gitCmd returns a git command run in root's working directory.
func gitCmd(root string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd
}

// gitRun runs git and returns its exit status.
func gitRun(root string, args ...string) error {
	return gitCmd(root, args...).Run()
}

// gitOutput runs git and returns its stdout.
func gitOutput(root string, args ...string) ([]byte, error) {
	return gitCmd(root, args...).Output()
}

// gitCombinedOutput runs git and returns its combined stdout/stderr.
func gitCombinedOutput(root string, args ...string) ([]byte, error) {
	return gitCmd(root, args...).CombinedOutput()
}

func isGitRepo(root string) bool {
	out, err := gitOutput(root, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func IsGitRepo(root string) bool {
	return isGitRepo(root)
}

func hasHead(root string) bool {
	return gitRun(root, "rev-parse", "--verify", "-q", "HEAD") == nil
}

func gitOutputLines(root string, args ...string) ([]string, error) {
	out, err := gitOutput(root, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func gitShow(root, spec string) (content []byte, ok bool) {
	out, err := gitOutput(root, "show", spec)
	if err != nil {
		return nil, false
	}
	return out, true
}
