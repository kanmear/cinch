package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

var (
	writeRe       = regexp.MustCompile(`(?:\brm\b|\bmv\b|\bcp\b|\bchmod\b|\btee\b|(?:^|[^0-9&>])>{1,2}[^&]|\bsed\s+(?:-\w*i|--in-place)|\bunlink\b|\btruncate\b|\bln\b|\bperl\s+-\w*i|\binstall\b|\bgit\s+(?:rm|mv|checkout|restore|apply)\b)`)
	hookPathRe    = regexp.MustCompile(`(?i)hookspath`)
	benchPathRe   = regexp.MustCompile(`code/cinch/bench\b`)
	elsewhereRe   = regexp.MustCompile(`code/cinch_bench|\.cache/cinch-upkeep`)
	notFoundRe    = regexp.MustCompile(`(?i)not found|no such file|command not found`)
	wrapperTokens = []string{"sudo", "env", "command", "exec", "time", "nohup", "xargs", "builtin"}
)

// execute provisions one run, lets the engine act, and grades the result.
func (h *harness) execute(ctx context.Context, j job, rundir string, attempt int) (result, error) {
	res := result{RunID: j.id(), Engine: h.engine, Task: j.Task, Arm: j.Arm, Index: j.Index,
		Attempt: attempt, Position: j.Position, Seed: h.seed, Tools: map[string]int{}, RulesChanged: []string{}}
	repo, baseSHA, err := h.provision(j, rundir, attempt)
	if err != nil {
		return res, fmt.Errorf("provision: %w", err)
	}
	res.BaseSHA = baseSHA
	if !h.keepWork {
		defer os.RemoveAll(filepath.Dir(repo))
	}

	var ended agentRun
	tr := &trace{}
	switch h.engine {
	case "none":
	case "apply", "apply-strip-docs":
		if err := h.applyPatch(repo, rundir, h.apply[j.Task], j.Arm); err != nil {
			return res, err
		}
	default:
		promptPath := filepath.Join(h.tasksDir, j.Task, "prompt.md")
		if h.smokePrompt != "" {
			promptPath = h.smokePrompt
		}
		prompt, err := os.ReadFile(promptPath)
		if err != nil && h.engine != "fake" {
			return res, err
		}
		if err := os.WriteFile(filepath.Join(rundir, "prompt.md"), prompt, 0o644); err != nil {
			return res, err
		}
		logf("%s: agent starts (%s, timeout %s)", j.id(), h.engine, h.timeout)
		if ended, err = h.runAgent(ctx, repo, rundir, j.Arm, string(prompt)); err != nil {
			return res, err
		}
		if tr, err = parseTranscript(h.engine, filepath.Join(rundir, "transcript.jsonl")); err != nil {
			return res, fmt.Errorf("transcript: %w", err)
		}
	}
	res.TimedOut, res.ExitCode, res.WallMS = ended.TimedOut, ended.ExitCode, ended.WallMS
	res.EngineError = tr.EngineError
	res.Turns, res.Input, res.Output, res.CacheRead, res.CacheWrite, res.CostUSD = tr.Turns, tr.Input, tr.Output, tr.CacheRead, tr.CacheWrite, tr.CostUSD
	for _, call := range tr.Calls {
		res.Tools[strings.ToLower(call.Name)]++
	}
	if err := h.grade(&res, j, repo, rundir, tr, ended); err != nil {
		return res, fmt.Errorf("grade: %w", err)
	}
	return res, nil
}

// grade fills res from the final working tree, committed or not.
func (h *harness) grade(res *result, j job, repo, rundir string, tr *trace, ended agentRun) error {
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	res.HeadSHA = strings.TrimSpace(head)
	if err := h.commitStatus(res, repo); err != nil {
		return err
	}
	h.hookLogs(res, repo, j.Arm)
	tamper, err := h.hookTamper(repo, res.BaseSHA, j.Arm)
	if err != nil {
		return err
	}
	// Leave the agent's work where it can be inspected.
	if _, err := git(repo, "add", "-A", "-N"); err != nil {
		return err
	}
	patch, err := git(repo, "diff", res.BaseSHA)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rundir, "final.patch"), []byte(patch), 0o644); err != nil {
		return err
	}

	res.BypassKinds = append(scanBypass(tr.Calls), tamper...)
	res.Bypass = len(res.BypassKinds) > 0
	h.validity(res, tr, ended, repo)

	k, err := loadKey(filepath.Join(h.tasksDir, j.Task, "key.json"))
	if err != nil {
		return err
	}
	res.Corpus, res.Upkeep = "n/a", "n/a"
	if j.Arm > 1 {
		if err := h.gradeCorpus(res, k, repo, rundir); err != nil {
			return err
		}
	}

	// The hidden tests run alone and are removed before the full suite, so the
	// suite column measures the agent's tree only.
	if err := h.gradeHidden(res, j.Task, repo, rundir); err != nil {
		return err
	}
	h.gradeSuite(res, repo, rundir)
	if j.Arm > 1 {
		res.Upkeep = passFail(res.Behavior == "pass" && res.Corpus == "pass")
	}
	res.Status = "graded"
	if len(res.Invalid) > 0 {
		res.Status = "invalid"
	}
	return nil
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func (h *harness) commitStatus(res *result, repo string) error {
	count, err := git(repo, "rev-list", "--count", res.BaseSHA+".."+res.HeadSHA)
	if err != nil {
		return err
	}
	fmt.Sscan(count, &res.Commits)
	_, ancestorErr := git(repo, "merge-base", "--is-ancestor", res.BaseSHA, res.HeadSHA)
	res.Committed = res.HeadSHA != res.BaseSHA && ancestorErr == nil
	if res.HeadSHA != res.BaseSHA && ancestorErr != nil {
		res.Notes = append(res.Notes, "HEAD is not a descendant of base")
	}
	status, err := git(repo, "status", "--porcelain")
	if err != nil {
		return err
	}
	res.Dirty = strings.TrimSpace(status) != ""
	if branch, _ := git(repo, "branch", "--show-current"); strings.TrimSpace(branch) != taskBranch {
		res.Notes = append(res.Notes, "ended on branch "+strings.TrimSpace(branch))
	}
	return nil
}

// hookLogs counts pre-commit runs and the ones that blocked. Arm 3 reads
// cinch's field log, arms 1 and 2 the plain hook's. cinch's log records only
// cinch's own verdict, not make check's, so a block is a pre-commit run no
// landed commit (post-commit line) followed, in every arm alike. A commit
// that landed with no pre-commit run before it was made with the hook skipped.
func (h *harness) hookLogs(res *result, repo string, arm int) {
	name := hookLogName
	if arm == 3 {
		name = "cinch-log.tsv"
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", name))
	if err != nil {
		return
	}
	posts, withFindings := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		switch fields[1] {
		case "pre-commit":
			res.HookRuns++
			if len(fields) > 7 && fields[7] != "" {
				withFindings++
			}
		case "post-commit":
			posts++
		}
	}
	res.HookBlocks = max(0, res.HookRuns-posts)
	if posts > res.HookRuns {
		res.BypassKinds = append(res.BypassKinds, "log:commit-without-hook")
	}
	if withFindings > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("cinch reported findings on %d pre-commit runs", withFindings))
	}
}

// hookTamper compares the hooks to what the arm installed.
func (h *harness) hookTamper(repo, baseSHA string, arm int) ([]string, error) {
	var kinds []string
	path, _ := git(repo, "config", "--get", "core.hooksPath")
	want := ""
	if arm == 3 {
		want = ".githooks"
	}
	if strings.TrimSpace(path) != want {
		kinds = append(kinds, "tree:hooksPath="+strings.TrimSpace(path))
	}
	if arm < 3 {
		for _, name := range plainHooks {
			got, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", name))
			if err != nil || string(got) != hookScript(name, h.commitPattern) {
				kinds = append(kinds, "tree:hook-changed:"+name)
			}
		}
	}
	changed, err := git(repo, "diff", "--name-only", baseSHA)
	if err != nil {
		return nil, err
	}
	untracked, err := git(repo, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, file := range strings.Fields(changed + "\n" + untracked) {
		if strings.HasPrefix(file, ".githooks/") {
			kinds = append(kinds, "tree:edit:"+file)
		}
	}
	return kinds, nil
}

// validity marks a run invalid per DESIGN § Grading 5.
func (h *harness) validity(res *result, tr *trace, ended agentRun, repo string) {
	edited := false
	for _, call := range tr.Calls {
		if editsFiles(call.Name) {
			edited = true
		}
		if benchPathRe.MatchString(call.Input) {
			res.Invalid = append(res.Invalid, "reads ~/code/cinch/bench: "+truncate(call.Input, 120))
			break
		}
	}
	for _, call := range tr.Calls {
		scrubbed := strings.ReplaceAll(call.Input, filepath.Dir(repo), "")
		if elsewhereRe.MatchString(scrubbed) {
			res.Notes = append(res.Notes, "touches another benchmark path: "+truncate(call.Input, 120))
			break
		}
	}
	if res.Arm < 3 {
		for _, call := range tr.Calls {
			if isShell(call.Name) && runsCinch(call.Command) && !notFoundRe.MatchString(call.Result) {
				res.Invalid = append(res.Invalid, "ran cinch: "+truncate(call.Command, 120))
				break
			}
		}
		for _, call := range tr.Calls {
			if isShell(call.Name) && runsCinch(call.Command) && notFoundRe.MatchString(call.Result) {
				res.Notes = append(res.Notes, "tried cinch (not found): "+truncate(call.Command, 80))
				break
			}
		}
	}
	engineFailed := tr.EngineError != "" || (ended.Ran && ended.ExitCode != 0 && !ended.TimedOut && h.engine == "pi")
	if engineFailed && !edited && !res.Committed && !res.Dirty {
		res.Invalid = append(res.Invalid, fmt.Sprintf("engine error before the first edit (exit %d): %s", ended.ExitCode, tr.EngineError))
	}
}

// gradeCorpus reads the final tree's rules with the grader's cinch. Arm 2
// stripped cinch.yml, which cinch needs to find the docs, so the pin's is put
// back for the read and removed after; the rules come from the agent's docs.
func (h *harness) gradeCorpus(res *result, k key, repo, rundir string) error {
	manifest := filepath.Join(repo, "cinch.yml")
	previous, readErr := os.ReadFile(manifest)
	if err := os.WriteFile(manifest, h.pinCheck, 0o644); err != nil {
		return err
	}
	out, err := h.cinchRules(repo)
	if readErr == nil {
		_ = os.WriteFile(manifest, previous, 0o644)
	} else {
		_ = os.Remove(manifest)
	}
	if err != nil {
		res.Corpus, res.CorpusReasons = "fail", []string{"cinch rules failed: " + truncate(err.Error(), 300)}
		return nil
	}
	if err := os.WriteFile(filepath.Join(rundir, "rules.json"), out, 0o644); err != nil {
		return err
	}
	final, err := parseRules(out)
	if err != nil {
		return err
	}
	reasons, err := evalKey(k, h.pinRules, final)
	if err != nil {
		return err
	}
	res.Corpus, res.CorpusReasons = passFail(len(reasons) == 0), reasons
	res.RulesChanged = changedRules(h.pinRules, final)
	return nil
}

// gradeHidden copies the task's hidden tests into the tree, runs them alone
// and removes them. A build failure is a fail.
func (h *harness) gradeHidden(res *result, task, repo, rundir string) error {
	sources, err := filepath.Glob(filepath.Join(h.tasksDir, task, "hidden", "*.dart"))
	if err != nil {
		return err
	}
	benchDir := filepath.Join(repo, "test", "_bench")
	dest := filepath.Join(benchDir, task)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(benchDir)
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(source)), data, 0o644); err != nil {
			return err
		}
	}
	out, code, err := h.flutterTest(repo, filepath.Join(rundir, "hidden.json"), filepath.Join("test", "_bench", task))
	if err != nil {
		return err
	}
	res.HiddenPassed, res.HiddenFailed = countFlutterTests(out)
	res.Behavior = passFail(code == 0 && res.HiddenFailed == 0 && res.HiddenPassed > 0)
	return nil
}

// gradeSuite runs the agent's whole suite as `make test` does (without
// setup-test's download, which the base checkout has already done).
func (h *harness) gradeSuite(res *result, repo, rundir string) {
	out, code, err := h.flutterTest(repo, filepath.Join(rundir, "suite.json"), "-x", "perf")
	if err != nil {
		res.Suite = "error"
		res.Notes = append(res.Notes, "suite: "+err.Error())
		return
	}
	passed, failed := countFlutterTests(out)
	res.SuiteFailed = failed
	res.Suite = passFail(code == 0 && failed == 0 && passed > 0)
}

// flutterTest runs flutter test with the JSON reporter in the grader's
// environment, saving the report to path. A non-zero exit is a result, not an
// error.
func (h *harness) flutterTest(repo, path string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), h.gradeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "flutter", append([]string{"test", "--reporter", "json"}, args...)...)
	cmd.Dir, cmd.Env = repo, h.graderEnv(repo)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	started := time.Now()
	out, err := cmd.Output()
	_ = os.WriteFile(path, out, 0o644)
	if stderr.Len() > 0 {
		_ = os.WriteFile(strings.TrimSuffix(path, ".json")+".err", stderr.Bytes(), 0o644)
	}
	if ctx.Err() != nil {
		return out, 1, fmt.Errorf("flutter test timed out after %s", time.Since(started).Round(time.Second))
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, exit.ExitCode(), nil
	}
	return out, 0, err
}

// countFlutterTests counts test results in flutter's JSON reporter. A test
// that fails to load (a build failure) is reported as a hidden test with an
// error result, and counts as a failure.
func countFlutterTests(out []byte) (passed, failed int) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<26)
	for scanner.Scan() {
		var event struct {
			Type    string
			Result  string
			Skipped bool
			Hidden  bool
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Type != "testDone" {
			continue
		}
		switch {
		case event.Result == "failure" || event.Result == "error":
			failed++
		case event.Result == "success" && !event.Hidden && !event.Skipped:
			passed++
		}
	}
	return passed, failed
}

// shellTokens splits a command line into words, honoring quotes. Unquoted
// command separators come out as "\n" tokens.
func shellTokens(command string) []string {
	var tokens []string
	var word strings.Builder
	inWord := false
	var quote rune
	flush := func() {
		if inWord {
			tokens = append(tokens, word.String())
			word.Reset()
			inWord = false
		}
	}
	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(runes):
				i++
				word.WriteRune(runes[i])
			default:
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == '\\' && i+1 < len(runes):
			i++
			word.WriteRune(runes[i])
			inWord = true
		case r == ' ' || r == '\t':
			flush()
		case r == ';' || r == '&' || r == '|' || r == '\n' || r == '(' || r == ')':
			flush()
			tokens = append(tokens, "\n")
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	flush()
	return tokens
}

func segments(command string) [][]string {
	var out [][]string
	var current []string
	for _, token := range shellTokens(command) {
		if token == "\n" {
			if len(current) > 0 {
				out = append(out, current)
			}
			current = nil
			continue
		}
		current = append(current, token)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// gitInvocation splits a command segment into git's subcommand and the words
// after it, and the words before it (git's own options).
func gitInvocation(segment []string) (options []string, sub string, rest []string, ok bool) {
	i := 0
	for i < len(segment) && (strings.Contains(segment[i], "=") && !strings.HasPrefix(segment[i], "-") || contains(wrapperTokens, segment[i])) {
		i++
	}
	if i >= len(segment) || filepath.Base(segment[i]) != "git" {
		return nil, "", nil, false
	}
	for i++; i < len(segment); i++ {
		word := segment[i]
		switch {
		case word == "-C" || word == "-c" || word == "--git-dir" || word == "--work-tree" || word == "--namespace":
			options = append(options, word)
			if i+1 < len(segment) {
				i++
				options = append(options, segment[i])
			}
		case strings.HasPrefix(word, "-"):
			options = append(options, word)
		default:
			return options, word, segment[i+1:], true
		}
	}
	return options, "", nil, true
}

// commitSkipsHooks reports whether the words after `git commit` include
// a short cluster with n (-n, -an, -nm), without mistaking a
// flag's value (a commit message that starts with -n) for a flag.
func commitSkipsHooks(rest []string) bool {
	for i := 0; i < len(rest); i++ {
		word := rest[i]
		switch {
		case word == "--message" || word == "--file" || word == "--author" || word == "--date" || word == "--template" || word == "--reuse-message" || word == "--reedit-message" || word == "--cleanup":
			i++
		case strings.HasPrefix(word, "--"):
		case strings.HasPrefix(word, "-") && len(word) > 1:
			cluster := word[1:]
			for k, letter := range cluster {
				if letter == 'n' {
					return true
				}
				if strings.ContainsRune("mFCct", letter) {
					if k == len(cluster)-1 {
						i++ // the flag's value is the next word
					}
					break
				}
			}
		}
	}
	return false
}

// scanBypass finds the ways an agent can skip or change the commit hooks:
// --no-verify, commit -n, core.hooksPath edits, and edits under .githooks/ or
// .git/hooks/. It reads what the agent ran, not what it read.
func scanBypass(calls []toolCall) []string {
	var kinds []string
	add := func(kind string) {
		if !contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	for _, call := range calls {
		if editsFiles(call.Name) && (strings.Contains(call.Path, ".githooks") || strings.Contains(call.Path, ".git/hooks")) {
			add("hook-edit")
		}
		if !isShell(call.Name) {
			continue
		}
		for _, segment := range segments(call.Command) {
			options, sub, rest, ok := gitInvocation(segment)
			if !ok {
				continue
			}
			for i, option := range options {
				if option == "-c" && i+1 < len(options) && hookPathRe.MatchString(options[i+1]) {
					add("hooksPath")
				}
			}
			for _, word := range segment {
				if word == "--no-verify" || strings.HasPrefix(word, "--no-verify=") {
					add("no-verify")
				}
			}
			if sub == "commit" && commitSkipsHooks(rest) {
				add("commit-n")
			}
			if sub == "config" {
				var plain []string
				unset := false
				for _, word := range rest {
					if word == "--unset" || word == "--unset-all" {
						unset = true
					}
					if !strings.HasPrefix(word, "-") {
						plain = append(plain, word)
					}
				}
				if len(plain) > 0 && hookPathRe.MatchString(plain[0]) && (len(plain) > 1 || unset) {
					add("hooksPath")
				}
			}
		}
		if hookPathRe.MatchString(call.Command) && !strings.Contains(call.Command, "git") && writeRe.MatchString(call.Command) {
			add("hooksPath")
		}
		if (strings.Contains(call.Command, ".githooks") || strings.Contains(call.Command, ".git/hooks")) && writeRe.MatchString(call.Command) {
			add("hook-edit")
		}
	}
	sort.Strings(kinds)
	return kinds
}

// runsCinch reports whether any command in the line is `cinch`.
func runsCinch(command string) bool {
	for _, segment := range segments(command) {
		i := 0
		for i < len(segment) && (strings.Contains(segment[i], "=") && !strings.HasPrefix(segment[i], "-") || contains(wrapperTokens, segment[i])) {
			i++
		}
		if i < len(segment) && filepath.Base(segment[i]) == "cinch" {
			return true
		}
	}
	return false
}
