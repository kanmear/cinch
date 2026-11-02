package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

const (
	armCode = "code" // docs and the pointer removed: the agent has only the code
	armDocs = "docs" // docs and the pointer kept, cinch removed
)

var canonicalArms = []string{armCode, armDocs}

var envStripped = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_CHILD_SESSION"}

// loadPins reads the KEY=VALUE pin file.
func loadPins(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pins: %w", err)
	}
	defer file.Close()
	pins := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("pins: %s: not KEY=VALUE: %q", path, line)
		}
		pins[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return pins, scanner.Err()
}

// fixturePin returns the ref and SHA PINS records for a repo, refusing a
// placeholder.
func fixturePin(pins map[string]string, repo string) (ref, sha string, err error) {
	prefix := pinPrefix(repo)
	ref, sha = pins[prefix+"_REF"], pins[prefix+"_SHA"]
	if sha == "" || sha == "TODO" || ref == "" || ref == "TODO" {
		return "", "", fmt.Errorf("PINS has no %s_REF/%s_SHA for %q (TODO): the owner fills them in before a run", prefix, prefix, repo)
	}
	return ref, sha, nil
}

// resolveRef returns the commit a fixture ref points at, peeling an annotated
// tag. It asks the remote rather than a checkout, so the answer is the
// fixture's own, whether it is a path or a URL.
func resolveRef(repo, ref string) (string, error) {
	out, err := git("", "ls-remote", repo, ref, ref+"^{}")
	if err != nil {
		return "", err
	}
	direct := map[string]string{}
	peeled := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		sha, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if base, found := strings.CutSuffix(name, "^{}"); found {
			peeled[base] = sha
		} else {
			direct[name] = sha
		}
	}
	if len(direct) != 1 {
		return "", fmt.Errorf("ref %q matches %d refs in %s, want exactly 1", ref, len(direct), repo)
	}
	for name, sha := range direct {
		if p, ok := peeled[name]; ok {
			return p, nil
		}
		return sha, nil
	}
	return "", nil
}

// fullRefName is the ref a fetch can name; ls-remote matched ref as a suffix.
func fullRefName(repo, ref string) (string, error) {
	out, err := git("", "ls-remote", repo, ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if _, name, ok := strings.Cut(line, "\t"); ok && !strings.HasSuffix(name, "^{}") {
			return name, nil
		}
	}
	return "", fmt.Errorf("ref %q not found in %s", ref, repo)
}

// setup verifies the pin and prepares the shared base checkout every run is
// copied from, then the sandbox.
func (h *harness) setup() error {
	sha, err := resolveRef(h.repo, h.ref)
	if err != nil {
		return err
	}
	if want := h.pinSHA; sha != want {
		return fmt.Errorf("fixture ref %s resolves to %s, but PINS records %s: refusing to run", h.ref, sha, want)
	}
	if h.questions.Commit != sha {
		return fmt.Errorf("the questions file is for commit %s, but the fixture is at %s: refusing to run", h.questions.Commit, sha)
	}
	h.baseEnv = cleanEnv(os.Environ())
	if err := h.prepareBase(); err != nil {
		return fmt.Errorf("base checkout: %w", err)
	}
	if err := h.resolveEngine(); err != nil {
		return err
	}
	return h.verifySandbox()
}

func cleanEnv(env []string) []string {
	var out []string
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !contains(envStripped, name) {
			out = append(out, entry)
		}
	}
	return out
}

// prepareBase fetches only the pinned commit's history (the fixture has later
// work an agent must not see), runs the repo's generation step once, and
// leaves the result for runs to copy. A lock file serializes processes.
func (h *harness) prepareBase() error {
	if err := os.MkdirAll(h.work, 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(h.work, "base.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	tag := pinPrefix(h.questions.Repo) + "-" + h.pinSHA[:12]
	h.baseDir = filepath.Join(h.work, "base-"+tag)
	marker := filepath.Join(h.work, "base-"+tag+".ok")
	// The marker records what the base was made with, so a changed -gen or
	// ObjectBox library rebuilds it.
	stamp := h.pinSHA + "\n" + h.gen + "\n" + h.objectbox + "\n"
	if got, err := os.ReadFile(marker); err != nil || string(got) != stamp {
		logf("preparing base checkout %s", h.baseDir)
		if err := os.RemoveAll(h.baseDir); err != nil {
			return err
		}
		if err := os.MkdirAll(h.baseDir, 0o755); err != nil {
			return err
		}
		refName, err := fullRefName(h.repo, h.ref)
		if err != nil {
			return err
		}
		for _, args := range [][]string{
			{"init", "-q"},
			{"fetch", "-q", "--no-tags", h.repo, refName},
			{"checkout", "-q", "--detach", h.pinSHA},
		} {
			if _, err := git(h.baseDir, args...); err != nil {
				return err
			}
		}
		_ = os.Remove(filepath.Join(h.baseDir, ".git", "FETCH_HEAD")) // names the fixture and the tag
		if head, err := git(h.baseDir, "rev-parse", "HEAD"); err != nil || strings.TrimSpace(head) != h.pinSHA {
			return fmt.Errorf("base checkout is at %q, want %s", strings.TrimSpace(head), h.pinSHA)
		}
		if h.objectbox != "" {
			if err := copyObjectBox(h.objectbox, filepath.Join(h.baseDir, ".objectbox", "lib")); err != nil {
				return err
			}
		}
		if h.gen != "" {
			// flutter gen-l10n writes build/l10n_untranslated.txt and fails without build/.
			if err := os.MkdirAll(filepath.Join(h.baseDir, "build"), 0o755); err != nil {
				return err
			}
			logf("%s in the base checkout", h.gen)
			genLog, err := os.Create(filepath.Join(h.work, "base-"+tag+".gen.log"))
			if err != nil {
				return err
			}
			defer genLog.Close()
			gen := exec.Command("/bin/sh", "-c", h.gen)
			gen.Dir, gen.Env, gen.Stdout, gen.Stderr = h.baseDir, h.genEnv(h.baseDir), genLog, genLog
			if err := gen.Run(); err != nil {
				return fmt.Errorf("%s (see %s): %w", h.gen, genLog.Name(), err)
			}
			if status, err := git(h.baseDir, "status", "--porcelain"); err != nil || strings.TrimSpace(status) != "" {
				return fmt.Errorf("base checkout is dirty after %s: %s", h.gen, status)
			}
		}
		if err := os.WriteFile(marker, []byte(stamp), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyObjectBox(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"libobjectbox.so", "VERSION"} {
		data, err := os.ReadFile(filepath.Join(from, name))
		if err != nil {
			return fmt.Errorf("objectbox lib: %w", err)
		}
		if err := os.WriteFile(filepath.Join(to, name), data, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// genEnv is the generation step's environment: the ordinary PATH plus
// Flutter when present, and the checkout's own ObjectBox library.
func (h *harness) genEnv(repo string) []string {
	path := os.Getenv("PATH")
	if h.flutter != "" {
		if _, err := os.Stat(h.flutter); err == nil {
			path = withDir(path, filepath.Join(h.flutter, "bin"))
		}
	}
	ld := filepath.Join(repo, ".objectbox", "lib")
	if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
		ld += ":" + existing
	}
	return setEnv(h.baseEnv, "PATH="+path, "LD_LIBRARY_PATH="+ld)
}

func withDir(path, dir string) string {
	for _, existing := range filepath.SplitList(path) {
		if existing == dir {
			return path
		}
	}
	return path + string(os.PathListSeparator) + dir
}

func setEnv(env []string, pairs ...string) []string {
	replaced := map[string]bool{}
	for _, pair := range pairs {
		name, _, _ := strings.Cut(pair, "=")
		replaced[name] = true
	}
	out := make([]string, 0, len(env)+len(pairs))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !replaced[name] {
			out = append(out, entry)
		}
	}
	return append(out, pairs...)
}

var agentsDocsHeadingRe = regexp.MustCompile(`(?m)^## Project docs\b.*$`)
var h2Re = regexp.MustCompile(`(?m)^## `)

// armSpec is what differs per repo in building an arm: where the docs live,
// what else goes in both arms, and, for a repo whose AGENTS.md is too long to
// derive from, the texts that replace it.
type armSpec struct {
	docsRoot string            // ".docs" or ".agent": the directory the docs live in
	strip    []string          // removed in both arms (agent tool config)
	agents   map[string]string // arm -> AGENTS.md text, when replaced rather than derived
}

// deriveAgents builds an arm's AGENTS.md from the pin's, as bench/upkeep does.
// The docs arm keeps every instruction except the cinch commands. The code
// arm keeps what is not about the docs: the "Project docs" section goes, and
// so does any sentence that points at the docs root.
func deriveAgents(pin, arm, docsRoot string) (string, error) {
	switch arm {
	case armDocs:
		var kept []string
		for _, line := range strings.Split(pin, "\n") {
			if strings.HasPrefix(line, "- ") && strings.Contains(line, "`cinch ") {
				continue
			}
			kept = append(kept, line)
		}
		out := strings.Replace(strings.Join(kept, "\n"), "## Project docs (cinch)", "## Project docs", 1)
		if !strings.Contains(out, docsRoot) {
			return "", fmt.Errorf("AGENTS.md has no pointer to %s for the docs arm to keep", docsRoot)
		}
		return out, nil
	case armCode:
		text := pin
		if loc := agentsDocsHeadingRe.FindStringIndex(text); loc != nil {
			end := len(text)
			if next := h2Re.FindStringIndex(text[loc[1]:]); next != nil {
				end = loc[1] + next[0]
			}
			text = text[:loc[0]] + text[end:]
		}
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if !strings.Contains(line, docsRoot) {
				continue
			}
			var pieces []string
			for _, piece := range strings.SplitAfter(line, ". ") {
				if !strings.Contains(piece, docsRoot) {
					pieces = append(pieces, piece)
				}
			}
			lines[i] = strings.TrimRight(strings.Join(pieces, ""), " ")
		}
		out := strings.Join(lines, "\n")
		if strings.Contains(out, docsRoot) || strings.Contains(strings.ToLower(out), "cinch") {
			return "", fmt.Errorf("AGENTS.md still mentions %s or cinch after the code arm's edit: %q", docsRoot, out)
		}
		return out, nil
	}
	return "", fmt.Errorf("unknown arm %q", arm)
}

// agentsFor is the arm's AGENTS.md: the spec's replacement if it has one,
// else derived from the pin's.
func (a armSpec) agentsFor(pin, arm string) (string, error) {
	if text, ok := a.agents[arm]; ok {
		return text, nil
	}
	return deriveAgents(pin, arm, a.docsRoot)
}

// armRemovals are the paths each arm deletes. scripts/cinch-log.sh is the
// field-log script only cinch's hooks call.
func (a armSpec) removals(arm string) []string {
	common := append([]string{"cinch.yml", ".githooks", "scripts/cinch-log.sh"}, a.strip...)
	switch arm {
	case armCode:
		return append([]string{a.docsRoot}, common...)
	case armDocs:
		return append([]string{a.docsRoot + "/workflows"}, common...)
	}
	return nil
}

// armSpecFor reads a repo's arm settings from PINS: <REPO>_DOCS (the docs
// root, default .docs), <REPO>_STRIP (comma-separated paths removed in both
// arms) and <REPO>_AGENTS_DIR (a directory, relative to PINS, holding code.md
// and docs.md that replace AGENTS.md).
func armSpecFor(pins map[string]string, pinsDir, repo string) (armSpec, error) {
	prefix := pinPrefix(repo)
	spec := armSpec{docsRoot: strings.Trim(pins[prefix+"_DOCS"], "/"), strip: splitList(pins[prefix+"_STRIP"])}
	if spec.docsRoot == "" {
		spec.docsRoot = ".docs"
	}
	if dir := pins[prefix+"_AGENTS_DIR"]; dir != "" {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(pinsDir, dir)
		}
		spec.agents = map[string]string{}
		for arm, name := range map[string]string{armCode: "code.md", armDocs: "docs.md"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return spec, fmt.Errorf("%s_AGENTS_DIR: %w", prefix, err)
			}
			spec.agents[arm] = string(data)
		}
	}
	return spec, nil
}

// provision copies the base checkout, applies the arm and makes it a fresh
// repository with one commit: the history of the fixture holds the docs the
// code arm must not have.
func (h *harness) provision(j job, attempt int) (string, error) {
	sum := sha256.Sum256([]byte(fmt.Sprint(h.seed, h.engine, j.id(), attempt)))
	root := filepath.Join(h.workspaces, hex.EncodeToString(sum[:5]))
	repo := filepath.Join(root, "project")
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if out, err := exec.Command("cp", "-a", "--reflink=auto", h.baseDir, repo).CombinedOutput(); err != nil {
		return "", fmt.Errorf("cp base: %v: %s", err, out)
	}
	if err := applyArm(repo, j.Arm, h.arms); err != nil {
		return "", fmt.Errorf("arm %s: %w", j.Arm, err)
	}
	return repo, nil
}

// applyArm turns a checkout of the pinned commit into an arm's workspace.
func applyArm(repo, arm string, spec armSpec) error {
	pin, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		return err
	}
	agents, err := spec.agentsFor(string(pin), arm)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		return err
	}
	for _, rel := range spec.removals(arm) {
		if err := os.RemoveAll(filepath.Join(repo, rel)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Dev"},
		{"config", "user.email", "dev@example.invalid"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "initial"},
	} {
		if _, err := git(repo, args...); err != nil {
			return err
		}
	}
	// The arm must hold.
	_, docs := os.Stat(filepath.Join(repo, spec.docsRoot))
	if (arm == armCode) != (docs != nil) {
		return fmt.Errorf("the %s directory is present=%t in the %s arm", spec.docsRoot, docs == nil, arm)
	}
	for _, rel := range append([]string{"cinch.yml", ".githooks", spec.docsRoot + "/workflows"}, spec.strip...) {
		if _, err := os.Stat(filepath.Join(repo, rel)); err == nil {
			return fmt.Errorf("%s survived the %s arm", rel, arm)
		}
	}
	if count, err := git(repo, "rev-list", "--count", "--all"); err != nil || strings.TrimSpace(count) != "1" {
		return fmt.Errorf("the %s arm's repository has %q commits, want 1", arm, strings.TrimSpace(count))
	}
	return nil
}
