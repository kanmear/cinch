package gitutil

import (
	"bufio"
	"os/exec"
	"strings"
)

func Cmd(repoRoot string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	return cmd
}

func Run(repoRoot string, args ...string) error {
	return Cmd(repoRoot, args...).Run()
}

func Output(repoRoot string, args ...string) ([]byte, error) {
	return Cmd(repoRoot, args...).Output()
}

func CombinedOutput(repoRoot string, args ...string) ([]byte, error) {
	return Cmd(repoRoot, args...).CombinedOutput()
}

func IsRepo(repoRoot string) bool {
	out, err := Output(repoRoot, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func HasHead(repoRoot string) bool {
	return Run(repoRoot, "rev-parse", "--verify", "-q", "HEAD") == nil
}

func OutputLines(repoRoot string, args ...string) ([]string, error) {
	out, err := Output(repoRoot, args...)
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

func ScannableFiles(repoRoot string) ([]string, error) {
	return OutputLines(repoRoot, "ls-files", "--cached", "--others", "--exclude-standard")
}

func Show(repoRoot, revisionSpec string) (content []byte, ok bool) {
	out, err := Output(repoRoot, "show", revisionSpec)
	if err != nil {
		return nil, false
	}
	return out, true
}
