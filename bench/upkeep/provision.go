package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

const (
	workspaceCommit = "chore: prepare workspace"
	taskBranch      = "feature/task"
	hookLogName     = "bench-hook.log"
)

var plainHooks = []string{"pre-commit", "commit-msg", "post-commit"}

var (
	agentsProjectRe = regexp.MustCompile(`(?m)^## Project\s*$`)
	envStripped     = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_CHILD_SESSION"}
)

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
	for _, key := range []string{"FIXTURE_REF", "FIXTURE_SHA", "CINCH_VERSION"} {
		if pins[key] == "" {
			return nil, fmt.Errorf("pins: %s has no %s", path, key)
		}
	}
	return pins, scanner.Err()
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

// setup verifies the pin and the grader's tools, and prepares the shared base
// checkout every run is copied from.
func (h *harness) setup() error {
	sha, err := resolveRef(h.repo, h.ref)
	if err != nil {
		return err
	}
	if want := h.pins["FIXTURE_SHA"]; sha != want {
		return fmt.Errorf("fixture ref %s resolves to %s, but PINS records %s: refusing to run", h.ref, sha, want)
	}
	h.pinSHA = sha

	if h.cinchBin, err = findCinch(h.cinch); err != nil {
		return err
	}
	version, err := exec.Command(h.cinchBin, "version").Output()
	if err != nil {
		return fmt.Errorf("%s version: %w", h.cinchBin, err)
	}
	if got := strings.TrimSpace(string(version)); got != h.pins["CINCH_VERSION"] {
		return fmt.Errorf("grader cinch %s is %s, but PINS records %s", h.cinchBin, got, h.pins["CINCH_VERSION"])
	}

	h.baseEnv = cleanEnv(os.Environ())
	if err := h.prepareBase(); err != nil {
		return fmt.Errorf("base checkout: %w", err)
	}
	if err := h.resolveEngine(); err != nil {
		return err
	}
	return h.verifySandboxes()
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

// findCinch picks the grader's own cinch. A mise shim is not a binary, so
// mise is asked where the real one is.
func findCinch(flagValue string) (string, error) {
	if flagValue != "" {
		abs, err := filepath.Abs(flagValue)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	if out, err := exec.Command("mise", "which", "cinch").Output(); err == nil {
		if path := strings.TrimSpace(string(out)); path != "" {
			return path, nil
		}
	}
	path, err := exec.LookPath("cinch")
	if err != nil {
		return "", fmt.Errorf("no cinch for the grader: pass -cinch")
	}
	return filepath.Abs(path)
}

// prepareBase fetches only the pinned commit's history (the fixture has later
// work an agent must not see), generates the gitignored sources once, and
// leaves the result for runs to copy. A lock file serializes the two engine
// processes.
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

	short := h.pinSHA[:12]
	h.baseDir = filepath.Join(h.work, "base-"+short)
	marker := filepath.Join(h.work, "base-"+short+".ok")
	rulesFile := filepath.Join(h.work, "pin-rules-"+short+".json")
	if _, err := os.Stat(marker); err != nil {
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
		if err := copyObjectBox(h.objectbox, filepath.Join(h.baseDir, ".objectbox", "lib")); err != nil {
			return err
		}
		// flutter gen-l10n writes build/l10n_untranslated.txt and fails without build/.
		if err := os.MkdirAll(filepath.Join(h.baseDir, "build"), 0o755); err != nil {
			return err
		}
		logf("make gen in the base checkout")
		genLog, err := os.Create(filepath.Join(h.work, "base-"+short+".gen.log"))
		if err != nil {
			return err
		}
		defer genLog.Close()
		gen := exec.Command("make", "gen")
		gen.Dir, gen.Env, gen.Stdout, gen.Stderr = h.baseDir, h.graderEnv(h.baseDir), genLog, genLog
		if err := gen.Run(); err != nil {
			return fmt.Errorf("make gen (see %s): %w", genLog.Name(), err)
		}
		if status, err := git(h.baseDir, "status", "--porcelain"); err != nil || strings.TrimSpace(status) != "" {
			return fmt.Errorf("base checkout is dirty after make gen: %s", status)
		}
		out, err := h.cinchRules(h.baseDir)
		if err != nil {
			return fmt.Errorf("pin rules: %w", err)
		}
		if err := os.WriteFile(rulesFile, out, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(marker, []byte(h.pinSHA+"\n"), 0o644); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return err
	}
	if h.pinRules, err = parseRules(data); err != nil {
		return err
	}
	if h.pinCheck, err = os.ReadFile(filepath.Join(h.baseDir, "cinch.yml")); err != nil {
		return err
	}
	h.commitPattern, err = commitPatternOf(h.pinCheck)
	return err
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

// graderEnv is the grader's and the base preparation's environment: the
// ordinary PATH plus Flutter, and the checkout's own ObjectBox library.
func (h *harness) graderEnv(repo string) []string {
	path := withFlutter(os.Getenv("PATH"), h.flutter)
	return setEnv(h.baseEnv, "PATH="+path, "LD_LIBRARY_PATH="+objectBoxPath(repo))
}

func objectBoxPath(repo string) string {
	path := filepath.Join(repo, ".objectbox", "lib")
	if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
		path += ":" + existing
	}
	return path
}

func withFlutter(path, flutterRoot string) string {
	bin := filepath.Join(flutterRoot, "bin")
	for _, dir := range filepath.SplitList(path) {
		if dir == bin {
			return path
		}
	}
	return path + string(os.PathListSeparator) + bin
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

// deriveAgents builds an arm's AGENTS.md from the pin's, as DESIGN § Arms
// says. Arm 2 keeps every instruction except the cinch commands. Arm 1 keeps
// the Project section only, minus the pointer to the docs it no longer has.
func deriveAgents(pin string, arm int) string {
	switch arm {
	case 1:
		loc := agentsProjectRe.FindStringIndex(pin)
		if loc == nil {
			return pin
		}
		project := strings.Replace(pin[loc[0]:], "Read `.docs/architecture.md` and `.docs/conventions.md` first. ", "", 1)
		return "# Agent instructions\n\n" + project
	case 2:
		var kept []string
		for _, line := range strings.Split(pin, "\n") {
			if strings.HasPrefix(line, "- ") && strings.Contains(line, "`cinch ") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Replace(strings.Join(kept, "\n"), "## Project docs (cinch)", "## Project docs", 1)
	}
	return pin
}

// stripMakeCheck drops the `cinch check` line from the check target, so the
// plain hook gates on the same non-cinch checks in arms 1 and 2.
func stripMakeCheck(makefile string) (string, error) {
	lines := strings.Split(makefile, "\n")
	var kept []string
	removed := 0
	for _, line := range lines {
		if line == "\tcinch check" {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed != 1 {
		return "", fmt.Errorf("Makefile has %d `cinch check` lines, want 1", removed)
	}
	return strings.Join(kept, "\n"), nil
}

// armRemovals are the paths each arm deletes, per DESIGN § Arms. scripts/
// cinch-log.sh is the field-log script only cinch's hooks call.
func armRemovals(arm int) []string {
	switch arm {
	case 1:
		return []string{".docs", "cinch.yml", ".githooks", "scripts/cinch-log.sh"}
	case 2:
		return []string{".docs/workflows", "cinch.yml", ".githooks", "scripts/cinch-log.sh"}
	}
	return nil
}

// hookScript is a plain hook of arms 1 and 2. pre-commit is the gate, make
// check; commit-msg enforces the same commit.pattern as cinch's commit-msg
// hook, with the same rejection text. All of them log a line, as cinch's field
// log does in arm 3, so blocked and skipped commits are countable in every arm.
func hookScript(name, commitPattern string) string {
	logLine := func(event, code string) string {
		return `printf '%s\t` + event + `\t%s\n' "$(date -Iseconds)" "` + code + `" >> "$(git rev-parse --git-common-dir)/` + hookLogName + `"`
	}
	switch name {
	case "post-commit":
		return "#!/bin/sh\n" + logLine("post-commit", "0") + "\n"
	case "commit-msg":
		quoted := strings.ReplaceAll(commitPattern, `'`, `'\''`)
		return `#!/bin/sh
# Rejects a commit whose subject does not match the repository's commit pattern.
pattern='` + quoted + `'
subject=$(head -n 1 "$1")
if printf '%s\n' "$subject" | grep -Eq -- "$pattern"; then
  code=0
else
  code=1
  printf 'commit: 1 finding\ncommit error %s:1: commit message does not match commit.pattern (%s): %s\n' "$1" "$pattern" "$subject" >&2
fi
` + logLine("commit-msg", "$code") + `
exit $code
`
	}
	return `#!/bin/sh
# Runs the repository's static gate (make check) before each commit.
make check
code=$?
` + logLine("pre-commit", "$code") + `
exit $code
`
}

var commitPatternRe = regexp.MustCompile(`(?m)^\s+pattern:\s*'((?:[^']|'')*)'\s*$`)

// commitPatternOf reads commit.pattern from a cinch.yml. The pin's is a
// single-quoted scalar; anything else is refused rather than guessed at.
func commitPatternOf(manifest []byte) (string, error) {
	m := commitPatternRe.FindSubmatch(manifest)
	if m == nil {
		return "", fmt.Errorf("cinch.yml has no single-quoted commit pattern")
	}
	return strings.ReplaceAll(string(m[1]), "''", "'"), nil
}

// provision copies the base checkout, applies the arm in one neutral commit,
// installs the arm's hooks and returns the checkout and its base SHA.
func (h *harness) provision(j job, rundir string, attempt int) (repo string, baseSHA string, err error) {
	sum := sha256.Sum256([]byte(fmt.Sprint(h.seed, h.engine, j.id(), attempt)))
	root := filepath.Join(h.workspaces, hex.EncodeToString(sum[:5]))
	repo = filepath.Join(root, "budgetron")
	if err := os.RemoveAll(root); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	if out, err := exec.Command("cp", "-a", "--reflink=auto", h.baseDir, repo).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("cp base: %v: %s", err, out)
	}
	if err := h.applyArm(repo, j.Arm); err != nil {
		return "", "", fmt.Errorf("arm %d: %w", j.Arm, err)
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

func (h *harness) applyArm(repo string, arm int) error {
	steps := [][]string{
		{"config", "user.name", "Budgetron Dev"},
		{"config", "user.email", "dev@budgetron.local"},
		{"config", "commit.gpgsign", "false"},
		{"checkout", "-q", "-b", taskBranch},
	}
	for _, args := range steps {
		if _, err := git(repo, args...); err != nil {
			return err
		}
	}
	if arm < 3 {
		pin, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
		if err != nil {
			return err
		}
		want, err := os.ReadFile(filepath.Join(h.armsDir, fmt.Sprintf("arm%d", arm), "AGENTS.md"))
		if err != nil {
			return err
		}
		if string(want) != deriveAgents(string(pin), arm) {
			return fmt.Errorf("%s/arm%d/AGENTS.md no longer matches what DESIGN derives from the pin's AGENTS.md", h.armsDir, arm)
		}
		if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), want, 0o644); err != nil {
			return err
		}
		makefile, err := os.ReadFile(filepath.Join(repo, "Makefile"))
		if err != nil {
			return err
		}
		edited, err := stripMakeCheck(string(makefile))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(repo, "Makefile"), []byte(edited), 0o644); err != nil {
			return err
		}
		args := append([]string{"rm", "-r", "-q", "--ignore-unmatch", "--"}, armRemovals(arm)...)
		if _, err := git(repo, args...); err != nil {
			return err
		}
	}
	if _, err := git(repo, "add", "-A"); err != nil {
		return err
	}
	if _, err := git(repo, "commit", "-q", "--allow-empty", "-m", workspaceCommit); err != nil {
		return err
	}
	if arm < 3 {
		for _, name := range plainHooks {
			if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", name), []byte(hookScript(name, h.commitPattern)), 0o755); err != nil {
				return err
			}
		}
	} else if _, err := git(repo, "config", "core.hooksPath", ".githooks"); err != nil {
		return err
	}
	// The arm must hold: no stray hooksPath in arms 1 and 2 (a global git
	// config could set one), the cinch hooks wired in arm 3.
	path, _ := git(repo, "config", "--get", "core.hooksPath")
	if want := map[bool]string{true: "", false: ".githooks"}[arm < 3]; strings.TrimSpace(path) != want {
		return fmt.Errorf("core.hooksPath is %q, want %q", strings.TrimSpace(path), want)
	}
	return nil
}

// cinchRules runs the grader's cinch in dir and returns its rule inventory.
func (h *harness) cinchRules(dir string) ([]byte, error) {
	cmd := exec.Command(h.cinchBin, "rules", "--json")
	cmd.Dir = dir
	cmd.Env = h.graderEnv(dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cinch rules --json: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !json.Valid(out) {
		return nil, fmt.Errorf("cinch rules --json: not JSON")
	}
	return out, nil
}
