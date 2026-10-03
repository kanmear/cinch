package gitutil

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

var (
	scannableFilesArgs = []string{"ls-files", "--cached", "--others", "--exclude-standard"}

	// repoLocalEnvVars is `git rev-parse --local-env-vars`: the variables that
	// pin a git invocation to one particular repository. A hook inherits some
	// of them from the commit that ran it — `git commit -a` and `git commit
	// <paths>` pass an absolute GIT_INDEX_FILE — so a git call aimed at a
	// different repository has to drop them, the way git itself does before
	// descending into a submodule.
	repoLocalEnvVars = map[string]bool{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_CONFIG":                       true,
		"GIT_CONFIG_PARAMETERS":            true,
		"GIT_CONFIG_COUNT":                 true,
		"GIT_OBJECT_DIRECTORY":             true,
		"GIT_DIR":                          true,
		"GIT_WORK_TREE":                    true,
		"GIT_IMPLICIT_WORK_TREE":           true,
		"GIT_GRAFT_FILE":                   true,
		"GIT_INDEX_FILE":                   true,
		"GIT_NO_REPLACE_OBJECTS":           true,
		"GIT_REPLACE_REF_BASE":             true,
		"GIT_PREFIX":                       true,
		"GIT_SHALLOW_FILE":                 true,
		"GIT_COMMON_DIR":                   true,
	}
)

func Cmd(repoRoot string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	return cmd
}

// ForeignCmd is Cmd for a repository other than the one cinch runs in (a
// rules.roots sibling), with the inherited repo-local environment removed.
func ForeignCmd(repoRoot string, args ...string) *exec.Cmd {
	cmd := Cmd(repoRoot, args...)
	cmd.Env = []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !repoLocalEnvVars[name] {
			cmd.Env = append(cmd.Env, entry)
		}
	}
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
	return commandLines(Cmd(repoRoot, args...))
}

func commandLines(cmd *exec.Cmd) ([]string, error) {
	out, err := cmd.Output()
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

// OutputPaths runs a git command that lists paths, with -z, and returns the
// names exactly as stored. Without -z git C-quotes any path holding non-ASCII
// bytes, '"', '\' or a control character (core.quotePath), and OutputLines'
// trimming would also eat a real leading or trailing space. An empty listing
// is nil, matching OutputLines.
func OutputPaths(repoRoot string, args ...string) ([]string, error) {
	return commandPaths(Cmd(repoRoot, nulSeparated(args)...))
}

// nulSeparated inserts -z right after the subcommand, where every git
// command that lists paths accepts it.
func nulSeparated(args []string) []string {
	if len(args) == 0 {
		return args
	}
	return append([]string{args[0], "-z"}, args[1:]...)
}

func commandPaths(cmd *exec.Cmd) ([]string, error) {
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range strings.Split(string(out), "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func ScannableFiles(repoRoot string) ([]string, error) {
	return OutputPaths(repoRoot, scannableFilesArgs...)
}

// ForeignScannableFiles is ScannableFiles for a repository other than the one
// cinch runs in; see ForeignCmd.
func ForeignScannableFiles(repoRoot string) ([]string, error) {
	return commandPaths(ForeignCmd(repoRoot, nulSeparated(scannableFilesArgs)...))
}

func Show(repoRoot, revisionSpec string) (content []byte, ok bool) {
	out, err := Output(repoRoot, "show", revisionSpec)
	if err != nil {
		return nil, false
	}
	return out, true
}
