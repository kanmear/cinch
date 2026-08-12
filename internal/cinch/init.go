package cinch

import (
	"os"
	"os/exec"
	"path/filepath"
)

// starterManifest is written by `cinch init` when no manifest exists yet.
const starterManifest = `paths:
  docs: .docs
  hooks: .githooks

# hooks:
#   pre-commit:
#     example:
#       run: make check
#       when: [src/]
`

// agentsWorkflowsLine and agentsIndexLine are the two lines a consumer's
// AGENTS.md needs: commands, not paths, so neither can go stale under a
// customized paths.docs.
const agentsWorkflowsLine = "Workflows: run `cinch workflows` to see what's available, `cinch workflow <name>` to load one."
const agentsIndexLine = "Docs: run `cinch index` to see every doc's path and title."

const starterAgentsMD = "# AGENTS.md\n\n" + agentsWorkflowsLine + "\n" + agentsIndexLine + "\n"

// CmdInit scaffolds a new cinch consumer: a manifest (if one doesn't already
// exist), the docs directory structure, a full render, and activated git
// hooks. Idempotent and non-destructive to anything authored — running it
// twice, or against an already-initialized repo, produces a byte-identical
// tree and touches nothing a human wrote.
func CmdInit(root string) int {
	manifestFile := filepath.Join(root, manifestPath)
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		if err := os.WriteFile(manifestFile, []byte(starterManifest), 0o644); err != nil {
			return Fail("init", err)
		}
		Step("wrote %s", manifestPath)
	} else {
		Step("%s already exists — left as-is", manifestPath)
	}

	m, err := loadManifest(root)
	if err != nil {
		return Fail("init", err)
	}

	plansDir := filepath.Join(root, docsPathValue(m), "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return Fail("init", err)
	}
	gitkeep := filepath.Join(plansDir, ".gitkeep")
	if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
		if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
			return Fail("init", err)
		}
	}
	Step("ensured %s", plansDir)

	if code := CmdRender(root); code != 0 {
		return code
	}

	if isGitRepo(root) {
		hooksDir := hooksPathValue(m)
		cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return Failf("init", "git config core.hooksPath failed: %v\n%s", err, out)
		}
		Step("activated hooks: core.hooksPath = %s", hooksDir)
	} else {
		Skip("init", "hooks", "not a git repository — hook shims generated but not activated")
	}

	agentsFile := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsFile); os.IsNotExist(err) {
		if err := os.WriteFile(agentsFile, []byte(starterAgentsMD), 0o644); err != nil {
			return Fail("init", err)
		}
		Step("wrote AGENTS.md")
	} else {
		Step("AGENTS.md already exists — add these lines if missing: %q, %q", agentsWorkflowsLine, agentsIndexLine)
	}

	return 0
}
