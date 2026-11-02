package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hookLogName is the field log cinch's own hook script writes into .git; it
// doubles as the bypass record (DESIGN § Fixture, Hooks).
const hookLogName = "cinch-log.tsv"

// readFileTrim reads a text file of a paragraph or two.
func readFileTrim(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// buildPrompt is the task's prompt.md, verbatim, with arm 3's instruction
// inserted as its own paragraph right before the tail (arms/prompt-tail.md).
// prompt.md already ends with the tail in every arm; a prompt that doesn't
// carry it exactly once at its end is refused, because the insertion point and
// the claim "the tail is in every arm" both rest on it.
func buildPrompt(prompt, tail, instruction string, arm int) (string, error) {
	if strings.Count(prompt, tail) != 1 || !strings.HasSuffix(strings.TrimSpace(prompt), tail) {
		return "", fmt.Errorf("prompt.md does not end with arms/prompt-tail.md exactly once")
	}
	if arm != 3 {
		return prompt, nil
	}
	if instruction == "" {
		return "", fmt.Errorf("arms/arm3-instruction.md is empty")
	}
	return strings.Replace(prompt, tail, instruction+"\n\n"+tail, 1), nil
}

// promptFor reads the task's prompt and applies the arm.
func (h *harness) promptFor(j job) (string, error) {
	dir, _, err := taskDir(h.leakDir, j.Task)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "prompt.md")
	if h.smokePrompt != "" {
		path = h.smokePrompt
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	prompt := string(data)
	if h.smokePrompt != "" {
		return prompt, nil
	}
	tail, err := readFileTrim(filepath.Join(h.leakDir, "arms", "prompt-tail.md"))
	if err != nil {
		return "", err
	}
	instruction, err := readFileTrim(filepath.Join(h.leakDir, "arms", "arm3-instruction.md"))
	if err != nil {
		return "", err
	}
	return buildPrompt(prompt, tail, instruction, j.Arm)
}

// provision copies the arm's built state (one commit on feature/task), wires
// the hooks, writes the run's database settings and returns the checkout and
// its base SHA. The agent never sees history: the state has exactly one commit.
func (h *harness) provision(j job, rundir string, attempt int, workURL string) (repo, baseSHA string, err error) {
	stateDir, err := h.ensureState(j.Task, j.Arm)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(fmt.Sprint(h.seed, h.engine, j.Pilot, j.id(), attempt)))
	root := filepath.Join(h.workspaces, hex.EncodeToString(sum[:5]))
	repo = filepath.Join(root, "project")
	if err := os.RemoveAll(root); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	if out, err := exec.Command("cp", "-a", "--reflink=auto", stateDir, repo).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("cp state: %v: %s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Dev"},
		{"config", "user.email", "dev@example.invalid"},
		{"config", "commit.gpgsign", "false"},
		{"config", "core.hooksPath", ".githooks"}, // what `cinch init` sets
	} {
		if _, err := git(repo, args...); err != nil {
			return "", "", err
		}
	}
	if err := checkWorkspace(repo); err != nil {
		return "", "", err
	}
	if err := writeEnvFile(repo, workURL); err != nil {
		return "", "", err
	}
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	baseSHA = strings.TrimSpace(head)
	if err := os.WriteFile(filepath.Join(rundir, "base.sha"), []byte(baseSHA+"\n"), 0o644); err != nil {
		return "", "", err
	}
	return repo, baseSHA, nil
}

// checkWorkspace asserts what the arm must hold: one commit, no other refs, no
// remote, the cinch hooks wired.
func checkWorkspace(repo string) error {
	if count, err := git(repo, "rev-list", "--count", "--all"); err != nil || strings.TrimSpace(count) != "1" {
		return fmt.Errorf("the workspace has %q commits, want 1", strings.TrimSpace(count))
	}
	if remotes, _ := git(repo, "remote"); strings.TrimSpace(remotes) != "" {
		return fmt.Errorf("the workspace has a remote: %s", remotes)
	}
	if msg, _ := git(repo, "log", "-1", "--format=%s"); strings.TrimSpace(msg) != workspaceCommit {
		return fmt.Errorf("the workspace's commit is %q", strings.TrimSpace(msg))
	}
	if branch, _ := git(repo, "branch", "--show-current"); strings.TrimSpace(branch) != taskBranch {
		return fmt.Errorf("the workspace is on branch %q, want %s", strings.TrimSpace(branch), taskBranch)
	}
	if path, _ := git(repo, "config", "--get", "core.hooksPath"); strings.TrimSpace(path) != ".githooks" {
		return fmt.Errorf("core.hooksPath is %q, want .githooks", strings.TrimSpace(path))
	}
	return nil
}

// writeEnvFile writes backend/.env (git-ignored by `.env*`) with the run's work
// database for both URLs, so a dev server started by the agent can't reach the
// DeltaDocs dev database on 5432 either.
func writeEnvFile(repo, workURL string) error {
	body := "# written by the benchmark harness; git-ignored\n" +
		"DATABASE_URL=" + workURL + "\n" +
		"TEST_DATABASE_URL=" + workURL + "\n" +
		"PORT=8080\nFRONTEND_URL=http://localhost:5173\nLOG_FORMAT=text\n"
	if err := os.WriteFile(filepath.Join(repo, "backend", ".env"), []byte(body), 0o600); err != nil {
		return err
	}
	if out, err := git(repo, "status", "--porcelain"); err != nil || strings.TrimSpace(out) != "" {
		return fmt.Errorf("backend/.env is not git-ignored (status: %q)", strings.TrimSpace(out))
	}
	return nil
}

// frontendDeps installs frontend/node_modules once per lockfile, outside the
// sandbox, and returns the directory to overlay under /work/frontend. "" when
// the state has no frontend or deps are disabled.
func (h *harness) frontendDeps(stateDir string) (string, error) {
	if h.noFrontendDeps {
		return "", nil
	}
	frontend := filepath.Join(stateDir, "frontend")
	lock, err := os.ReadFile(filepath.Join(frontend, "bun.lock"))
	if err != nil {
		return "", nil
	}
	pkg, err := os.ReadFile(filepath.Join(frontend, "package.json"))
	if err != nil {
		return "", nil
	}
	sum := sha256.Sum256(append(lock, pkg...))
	key := hex.EncodeToString(sum[:6])
	h.nodeMu.Lock()
	defer h.nodeMu.Unlock()
	if dir, ok := h.nodeDirs[key]; ok {
		return dir, nil
	}
	root := filepath.Join(h.work, "node", key)
	modules := filepath.Join(root, "frontend", "node_modules")
	marker := filepath.Join(root, ".ok")
	if _, err := os.Stat(marker); err != nil {
		logf("installing frontend dependencies (bun install --frozen-lockfile) for lockfile %s", key)
		if err := os.RemoveAll(root); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
			return "", err
		}
		for _, name := range []string{"package.json", "bun.lock", "bunfig.toml", ".npmrc"} {
			if data, err := os.ReadFile(filepath.Join(frontend, name)); err == nil {
				if err := os.WriteFile(filepath.Join(root, "frontend", name), data, 0o644); err != nil {
					return "", err
				}
			}
		}
		cmd := exec.Command(h.sandbox.bun, "install", "--frozen-lockfile")
		cmd.Dir, cmd.Env = filepath.Join(root, "frontend"), h.baseEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("bun install: %v\n%s", err, truncate(string(out), 2000))
		}
		if err := os.WriteFile(marker, []byte(key+"\n"), 0o644); err != nil {
			return "", err
		}
	}
	if h.nodeDirs == nil {
		h.nodeDirs = map[string]string{}
	}
	h.nodeDirs[key] = modules
	return modules, nil
}
