package cinch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// starterManifest is written by `cinch init` when no manifest exists yet.
const starterManifest = `paths.docs  = .docs
paths.hooks = .githooks

# hooks.pre-commit.example.run  = make check
# hooks.pre-commit.example.when = src/
`

// agentsWorkflowsLine and agentsDocsLine are the two lines a consumer's
// AGENTS.md needs: commands, not paths, so neither can go stale under a
// customized paths.docs.
const agentsWorkflowsLine = "Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one."
const agentsDocsLine = "Docs: run `cinch docs` to see every doc's path and title."

const starterAgentsMD = "# AGENTS.md\n\n" + agentsWorkflowsLine + "\n" + agentsDocsLine + "\n"

// CmdInit scaffolds a new cinch consumer: a manifest (if one doesn't already
// exist), the docs directory structure, a full render, and activated git
// hooks. Idempotent and non-destructive to anything authored — running it
// twice, or against an already-initialized repo, produces a byte-identical
// tree and touches nothing a human wrote.
func CmdInit(root string) int {
	manifestFile := filepath.Join(root, manifestPath)
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		if err := os.WriteFile(manifestFile, []byte(starterManifest), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
			return 1
		}
		fmt.Printf("wrote %s\n", manifestPath)
	}

	m, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}

	plansDir := filepath.Join(root, docsPathValue(m), "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
		return 1
	}
	gitkeep := filepath.Join(plansDir, ".gitkeep")
	if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
		if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
			return 1
		}
	}

	if code := CmdRender(root); code != 0 {
		return code
	}

	if isGitRepo(root) {
		hooksDir := hooksPathValue(m)
		cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "cinch: git config core.hooksPath failed: %v\n%s", err, out)
			return 1
		}
		fmt.Printf("activated hooks: core.hooksPath = %s\n", hooksDir)
	} else {
		fmt.Fprintln(os.Stderr, "cinch: not a git repository — hook shims generated but not activated")
	}

	agentsFile := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsFile); os.IsNotExist(err) {
		if err := os.WriteFile(agentsFile, []byte(starterAgentsMD), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "cinch: "+err.Error())
			return 1
		}
		fmt.Println("wrote AGENTS.md")
	} else {
		fmt.Println("AGENTS.md already exists — add these lines if they're missing:")
		fmt.Println("  " + agentsWorkflowsLine)
		fmt.Println("  " + agentsDocsLine)
	}

	return 0
}
