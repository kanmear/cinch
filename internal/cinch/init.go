package cinch

import (
	"os"
	"os/exec"
	"path/filepath"

	"cinch/internal/output"
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

// starterManifestExample is written by `cinch init` when no manifest.example.yml
// exists yet — a commented starting point for the "richer project manifest"
// convention (see README.md), which cinch never reads or validates.
const starterManifestExample = `# manifest.example.yml — project-owned, cinch never reads this file.
#
# cinch.yml only binds what cinch itself reads: paths.*, hooks.*,
# commit.pattern. Workflow-template *prose*, though, can reference
# arbitrary project-specific values cinch never touches — commands, ports,
# service layout, test taxonomy. A project with enough of those is better
# served by its own separate, schema-free file than by overloading
# cinch.yml. There's no cinch convention format for this (by design —
# cinch binds values, not shape): rename this file, restructure it, or
# delete it — nothing here is enforced or read by cinch. The convention
# name is manifest.yml, deliberately distinct from cinch.yml so the two
# are never confused.
#
# Reference it from your own docs/workflow prose by hand, e.g. "run the
# command named test-backend under development.commands in manifest.yml" —
# cinch's {{key}} substitution never sees or resolves against this file.

project:
  name: my-app
services:
  backend:  { path: backend,  port: 8080 }
  frontend: { path: frontend, port: 5173 }
development:
  commands:
    test-backend:  make test-backend
    test-frontend: make test-frontend
taxonomy:
  test_tiers:
    - { id: unit, cmd: test-backend }
`

// CmdInit scaffolds a new cinch consumer: a manifest (if one doesn't already
// exist), the docs directory structure, a full render, and activated git
// hooks. Idempotent and non-destructive to anything authored — running it
// twice, or against an already-initialized repo, produces a byte-identical
// tree and touches nothing a human wrote.
func CmdInit(root string) int {
	manifestFile := filepath.Join(root, manifestPath)
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		if err := os.WriteFile(manifestFile, []byte(starterManifest), 0o644); err != nil {
			return output.Fail("init", err)
		}
		output.Step("wrote %s", manifestPath)
	} else {
		output.Step("%s already exists — left as-is", manifestPath)
	}

	manifestExampleFile := filepath.Join(root, "manifest.example.yml")
	if _, err := os.Stat(manifestExampleFile); os.IsNotExist(err) {
		if err := os.WriteFile(manifestExampleFile, []byte(starterManifestExample), 0o644); err != nil {
			return output.Fail("init", err)
		}
		output.Step("wrote manifest.example.yml")
	} else {
		output.Step("manifest.example.yml already exists — left as-is")
	}

	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("init", err)
	}

	plansDir := filepath.Join(root, docsPathValue(m), "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return output.Fail("init", err)
	}
	gitkeep := filepath.Join(plansDir, ".gitkeep")
	if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
		if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
			return output.Fail("init", err)
		}
	}
	output.Step("ensured %s", plansDir)

	if code := CmdRender(root); code != 0 {
		return code
	}

	if isGitRepo(root) {
		hooksDir := hooksPathValue(m)
		cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return output.Failf("init", "git config core.hooksPath failed: %v\n%s", err, out)
		}
		output.Step("activated hooks: core.hooksPath = %s", hooksDir)
	} else {
		output.Skip("init", "hooks", "not a git repository — hook shims generated but not activated")
	}

	agentsFile := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsFile); os.IsNotExist(err) {
		if err := os.WriteFile(agentsFile, []byte(starterAgentsMD), 0o644); err != nil {
			return output.Fail("init", err)
		}
		output.Step("wrote AGENTS.md")
	} else {
		output.Step("AGENTS.md already exists — add these lines if missing: %q, %q", agentsWorkflowsLine, agentsIndexLine)
	}

	return 0
}
