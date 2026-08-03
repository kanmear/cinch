// Command cinch renders project-bound docs from templates and checks the
// harness for drift. It never parses project source: type and route
// introspection is delegated to commands declared in the manifest.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const usage = `cinch — agent harness tooling

usage:
  cinch render    render .agent/templates/**.md -> .agent/workflows/**.md
  cinch index     regenerate .agent/index.md
  cinch check     run all harness checks (exit 1 on error)

paths are resolved from the git repository root.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		os.Exit(1)
	}

	switch os.Args[1] {
	case "render":
		err = cmdRender(root)
	case "index":
		err = cmdIndex(root)
	case "check":
		err = cmdCheck(root)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		os.Exit(1)
	}
}

// repoRoot returns the git repository root.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// agentDir returns the absolute path of the .agent directory.
func agentDir(root string) string { return filepath.Join(root, ".agent") }
