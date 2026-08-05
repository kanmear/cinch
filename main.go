// Command cinch renders project-bound docs from templates and checks the
// harness for drift. It never parses project source: type and route
// introspection is delegated to commands declared in the manifest.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cinch/internal/check"
	"cinch/internal/index"
	"cinch/internal/render"
	"cinch/internal/version"
)

const usage = `cinch — agent harness tooling

usage:
  cinch render      render .agent/templates/**.md -> .agent/workflows/**.md
  cinch index       regenerate .agent/index.md
  cinch check       run all harness checks (exit 1 on error)
  cinch ignores     list every rule declared cinch:ignore in domain/*.md
  cinch version     print the version (x.y.z[letter])
  cinch version bump <major|minor|patch|hotfix>
                    rewrite version/ and print old -> new

paths are resolved from the git repository root; the version is resolved next
to the binary (version/ in the cinch clone), so it works from any directory.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// version is binary-relative and needs no git repository.
	if os.Args[1] == "version" || os.Args[1] == "--version" {
		if err := version.Run(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
			os.Exit(1)
		}
		return
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		os.Exit(1)
	}

	switch os.Args[1] {
	case "render":
		err = render.Run(root)
	case "index":
		err = index.Run(root)
	case "check":
		err = check.Run(root)
	case "ignores":
		err = check.Ignores(root)
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
