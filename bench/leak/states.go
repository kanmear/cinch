package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	workspaceCommit = "chore: prepare workspace"
	taskBranch      = "feature/task"
	stateAuthor     = "bench"
	stateEmail      = "bench@localhost"
	stateDate       = "2026-01-01T00:00:00Z"
)

// stateHeaderRe is the optional header of a state script, in its first lines:
//
//	# leak-state: base=FIX
//
// base=BASE (the default) exports T_BASE_SHA; base=FIX exports T_FIX_SHA (T1's
// arm 2 is the fix commit with the task stripped).
var stateHeaderRe = regexp.MustCompile(`(?m)^#\s*leak-state:\s*base=(BASE|FIX)\s*$`)

// stateKind is how a state is built: its script and which commit it exports.
type stateKind struct {
	Task   string
	Arm    int // 1 or 2
	Script string
	Base   string // BASE or FIX
}

func (h *harness) stateKindOf(task string, arm int) (stateKind, error) {
	arm = stateArm(arm)
	name := fmt.Sprintf("%s-arm%d.py", task, arm)
	candidates := []string{filepath.Join(h.leakDir, "states", name), filepath.Join(h.leakDir, "private", "states", name)}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		head := string(data)
		if len(head) > 2000 {
			head = head[:2000]
		}
		base := "BASE"
		if m := stateHeaderRe.FindStringSubmatch(head); m != nil {
			base = m[1]
		}
		return stateKind{Task: task, Arm: arm, Script: path, Base: base}, nil
	}
	return stateKind{}, fmt.Errorf("no state script %s (looked in %s)", name, strings.Join(candidates, ", "))
}

// fixtureGit runs a read-only git command against the private fixture. The only
// verbs used are archive, show, ls-tree and rev-parse.
func (h *harness) fixtureGit(args ...string) ([]byte, error) {
	switch args[0] {
	case "archive", "show", "ls-tree", "rev-parse", "cat-file":
	default:
		return nil, fmt.Errorf("refusing git %s on the fixture: it is read-only", args[0])
	}
	cmd := exec.Command("git", append([]string{"-C", h.fixture}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// resolveSHAs checks every BASE and FIX SHA in PINS names a commit of the
// fixture, by its full SHA.
func (h *harness) resolveSHAs(tasks []string) error {
	for _, task := range tasks {
		for _, kind := range []string{"BASE", "FIX"} {
			key := fmt.Sprintf("%s_%s_SHA", task, kind)
			want := h.pins[key]
			if want == "" {
				if kind == "FIX" {
					continue // T4 is unfixed
				}
				return fmt.Errorf("PINS has no %s", key)
			}
			out, err := h.fixtureGit("rev-parse", "--verify", want+"^{commit}")
			if err != nil {
				return fmt.Errorf("%s=%s does not resolve in the fixture: %w", key, want, err)
			}
			if got := strings.TrimSpace(string(out)); got != want {
				return fmt.Errorf("%s=%s resolves to %s: pin a full SHA", key, want, got)
			}
		}
	}
	return nil
}

// cleanState builds a task's state into dir (which must not exist) and returns
// the sha256 of `git ls-tree -r HEAD`. See BUILD.md § 2.
func (h *harness) buildState(kind stateKind, dir string) (hash string, err error) {
	base := h.pins[kind.Task+"_BASE_SHA"]
	fix := h.pins[kind.Task+"_FIX_SHA"]
	export := base
	if kind.Base == "FIX" {
		export = fix
		if export == "" {
			return "", fmt.Errorf("%s script exports the fix commit, but PINS has no %s_FIX_SHA", kind.Script, kind.Task)
		}
	}
	if base == "" {
		return "", fmt.Errorf("PINS has no %s_BASE_SHA", kind.Task)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// 1. the export
	archive := exec.Command("git", "-C", h.fixture, "archive", export)
	untar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return "", err
	}
	untar.Stdin = pipe
	var archErr, untarErr bytes.Buffer
	archive.Stderr, untar.Stderr = &archErr, &untarErr
	if err := untar.Start(); err != nil {
		return "", err
	}
	if err := archive.Run(); err != nil {
		_ = untar.Wait()
		return "", fmt.Errorf("git archive %s: %v: %s", export, err, archErr.String())
	}
	if err := untar.Wait(); err != nil {
		return "", fmt.Errorf("tar: %v: %s", err, untarErr.String())
	}
	// 2 and 3. the common preparation and the task's own script
	env := append(cleanEnv(os.Environ()),
		"PYTHONDONTWRITEBYTECODE=1", "DELTADOCS="+h.fixture, "BASE_SHA="+base, "FIX_SHA="+fix, "TASK="+kind.Task, fmt.Sprintf("ARM=%d", kind.Arm), "LEAK_EXPORT_SHA="+export)
	for _, script := range []string{filepath.Join(h.leakDir, "states", "common.py"), kind.Script} {
		cmd := exec.Command("python3", "-I", "-B", script, dir)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("%s: %v\n%s", script, err, truncate(string(out), 4000))
		}
	}
	// 4. render with the pinned cinch
	if err := h.cinchIn(dir, "render"); err != nil {
		return "", err
	}
	// 5. one commit
	author := []string{"GIT_AUTHOR_NAME=" + stateAuthor, "GIT_AUTHOR_EMAIL=" + stateEmail, "GIT_AUTHOR_DATE=" + stateDate,
		"GIT_COMMITTER_NAME=" + stateAuthor, "GIT_COMMITTER_EMAIL=" + stateEmail, "GIT_COMMITTER_DATE=" + stateDate,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	gitEnvList := append(cleanEnv(os.Environ()), author...)
	for _, args := range [][]string{
		{"init", "-q", "-b", taskBranch},
		{"config", "user.name", "Dev"},
		{"config", "user.email", "dev@example.invalid"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", workspaceCommit},
	} {
		if _, err := gitEnv(dir, gitEnvList, args...); err != nil {
			return "", err
		}
	}
	// 4b. cinch check must be green on the committed state
	if err := h.cinchIn(dir, "check"); err != nil {
		return "", err
	}
	return treeHash(dir)
}

// treeHash is the sha256 of `git ls-tree -r HEAD`: every path, mode and blob.
func treeHash(dir string) (string, error) {
	out, err := git(dir, "ls-tree", "-r", "HEAD")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(out))
	return hex.EncodeToString(sum[:]), nil
}

// cinchIn runs the grader's cinch in dir, failing with its output.
func (h *harness) cinchIn(dir string, args ...string) error {
	cmd := exec.Command(h.cinchBin, args...)
	cmd.Dir = dir
	cmd.Env = h.graderEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cinch %s in the state: %v\n%s", strings.Join(args, " "), err, truncate(string(out), 4000))
	}
	return nil
}

// stateKey names a built state in the cache.
func stateKey(task string, arm int) string { return fmt.Sprintf("%s-arm%d", task, stateArm(arm)) }

// ensureState builds a state once per invocation, verifies its tree hash
// against PINS (unless unpinned is allowed, in which case the hash is only
// logged) and returns the directory every run is copied from.
func (h *harness) ensureState(task string, arm int) (string, error) {
	key := stateKey(task, arm)
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if dir, ok := h.states[key]; ok {
		return dir, nil
	}
	kind, err := h.stateKindOf(task, arm)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(h.work, "states", key)
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	logf("building state %s", key)
	hash, err := h.buildState(kind, dir)
	if err != nil {
		return "", fmt.Errorf("state %s: %w", key, err)
	}
	pinKey := treePinKey(task, arm)
	if pinTODO(h.pins, pinKey) {
		if !h.allowUnpinned {
			return "", fmt.Errorf("PINS has no %s (TODO); `leak states -pin` prints it. State %s hashes to %s", pinKey, key, hash)
		}
		logf("state %s: tree sha256 %s (unpinned)", key, hash)
	} else if want := h.pins[pinKey]; hash != want {
		return "", fmt.Errorf("state %s drifted: its tree hashes to %s, but PINS records %s=%s", key, hash, pinKey, want)
	}
	if h.states == nil {
		h.states = map[string]string{}
	}
	h.states[key] = dir
	h.stateHashes[key] = hash
	return dir, nil
}

// statesCommand is `leak states`: build one or all states, print the tree
// hashes (-pin prints them as PINS lines).
func statesCommand(h *harness, tasks []string, arms []int, out string, pin bool) error {
	for _, task := range tasks {
		for _, arm := range arms {
			if _, err := h.stateKindOf(task, arm); err != nil {
				continue // this task has no such state (arm 3 shares arm 1; T4 may have no arm 2)
			}
			dir, err := h.ensureState(task, arm)
			if err != nil {
				return err
			}
			hash := h.stateHashes[stateKey(task, arm)]
			if out != "" {
				dest := filepath.Join(out, stateKey(task, arm))
				if err := os.RemoveAll(dest); err != nil {
					return err
				}
				if err := os.MkdirAll(out, 0o755); err != nil {
					return err
				}
				if b, err := exec.Command("cp", "-a", "--reflink=auto", dir, dest).CombinedOutput(); err != nil {
					return fmt.Errorf("cp state: %v: %s", err, b)
				}
			}
			if pin {
				fmt.Printf("%s=%s\n", treePinKey(task, arm), hash)
			} else {
				fmt.Printf("%s  %s\n", hash, stateKey(task, arm))
			}
		}
	}
	return nil
}

// readLines is a small helper for files of one value per line.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}
