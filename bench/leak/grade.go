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
	// A transcript reading either of these is invalid (DESIGN § Grading 5).
	benchPathRe = regexp.MustCompile(`code/cinch/bench\b|code/project_deltadocs\b`)
	elsewhereRe = regexp.MustCompile(`code/cinch_bench|\.cache/cinch-(?:leak|upkeep|recover|binding)`)
	portUsedRe  = regexp.MustCompile(`(?i)address already in use`)
	noDBRe      = regexp.MustCompile(`cannot reach the test database`)
	devDBRe     = regexp.MustCompile(`\bdev-db\b`)
	port5432Re  = regexp.MustCompile(`[:\s=]5432\b`)
	planFlowRe  = regexp.MustCompile(`dev-plan-feature`)
	ignoreAddRe = regexp.MustCompile(`(?m)^\+.*cinch:ignore`)
)

// execute provisions one run, lets the engine act, and grades the result.
func (h *harness) execute(ctx context.Context, j job, rundir string, attempt int) (res result, err error) {
	res = result{RunID: j.id(), Engine: h.engine, Task: j.Task, Arm: j.Arm, Index: j.Index, Pilot: j.Pilot,
		Attempt: attempt, Position: j.Position, Seed: h.seed, Tools: map[string]int{}, Tests: map[string]string{}}
	capDur, err := h.capFor(j)
	if err != nil {
		return res, err
	}
	res.CapMin = int(capDur / time.Minute)

	workDB, gradeDB := runDBName(h.engine, j, attempt, "work"), runDBName(h.engine, j, attempt, "grade")
	for _, db := range []string{workDB, gradeDB} {
		if err := h.pg.createDB(db); err != nil {
			return res, fmt.Errorf("databases: %w", err)
		}
	}
	defer func() {
		if h.keepDBs || res.Status == "invalid" {
			return
		}
		for _, db := range []string{workDB, gradeDB} {
			_ = h.pg.dropDB(db)
		}
	}()
	workURL, gradeURL, err := h.runURLs(j.Task, j.Arm, workDB, gradeDB)
	if err != nil {
		return res, err
	}

	repo, baseSHA, err := h.provision(j, rundir, attempt, workURL)
	if err != nil {
		return res, fmt.Errorf("provision: %w", err)
	}
	res.BaseSHA, res.StateTree = baseSHA, h.stateHashes[stateKey(j.Task, j.Arm)]
	if !h.keepWork {
		defer os.RemoveAll(filepath.Dir(repo))
	}
	nodeModules, err := h.frontendDeps(h.states[stateKey(j.Task, j.Arm)])
	if err != nil {
		return res, err
	}
	env := runEnv{dbURL: workURL, nodeModules: nodeModules}

	var ended agentRun
	tr := &trace{}
	switch h.engine {
	case "none":
	case "apply", "apply-strip-docs":
		if err := h.applyPatch(repo, rundir, h.apply[j.Task], env); err != nil {
			return res, err
		}
	default:
		prompt, err := h.promptFor(j)
		if err != nil && h.engine != "fake" {
			return res, err
		}
		if err := os.WriteFile(filepath.Join(rundir, "prompt.md"), []byte(prompt), 0o644); err != nil {
			return res, err
		}
		logf("%s: agent starts (%s, cap %s)", j.id(), h.engine, capDur)
		if ended, err = h.runAgent(ctx, repo, rundir, env, capDur, prompt); err != nil {
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
	if err := h.grade(&res, j, repo, rundir, tr, ended, gradeDB, gradeURL); err != nil {
		return res, fmt.Errorf("grade: %w", err)
	}
	return res, nil
}

// runURLs are the work and grade databases' URLs, from the state's template.
func (h *harness) runURLs(task string, arm int, workDB, gradeDB string) (string, string, error) {
	dir, err := h.ensureState(task, arm)
	if err != nil {
		return "", "", err
	}
	template, err := testURLTemplate(dir)
	if err != nil {
		return "", "", err
	}
	work, err := databaseURL(template, workDB, h.pg.port)
	if err != nil {
		return "", "", err
	}
	grade, err := databaseURL(template, gradeDB, h.pg.port)
	return work, grade, err
}

// grade fills res from the final working tree, committed or not.
func (h *harness) grade(res *result, j job, repo, rundir string, tr *trace, ended agentRun, gradeDB, gradeURL string) error {
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	res.HeadSHA = strings.TrimSpace(head)
	if err := h.commitStatus(res, repo); err != nil {
		return err
	}
	h.hookLogs(res, repo, rundir)
	tamper, err := h.hookTamper(repo, res.BaseSHA)
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
	res.IgnoreAdded = len(ignoreAddRe.FindAllString(patch, -1))

	res.BypassKinds = append(append(scanBypass(tr.Calls), tamper...), res.BypassKinds...)
	sort.Strings(res.BypassKinds)
	res.Bypass = len(res.BypassKinds) > 0
	h.scanTranscript(res, tr, ended, repo)

	// The hidden package runs alone on a fresh database; the rule checks read the
	// tree; the full backend suite runs last on another fresh database.
	if err := h.gradeHidden(res, j.Task, repo, rundir, gradeDB, gradeURL); err != nil {
		return err
	}
	if err := h.gradeRules(res, j, repo, rundir); err != nil {
		return err
	}
	if !h.noSuite && !calibrationOnly(h.engine) {
		h.gradeSuite(res, repo, rundir, gradeDB, gradeURL)
	}
	res.Status = "graded"
	if len(res.Invalid) > 0 {
		res.Status = "invalid"
	}
	return nil
}

// calibrationOnly engines skip the suite unless asked: they exist to check the
// hidden tests and the rule checks.
func calibrationOnly(engine string) bool {
	return engine == "none" || engine == "apply" || engine == "apply-strip-docs"
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

// hookTrace counts the hooks git ran in the sandbox, from its trace2 event log
// (sandbox.go): pre-commit, commit-msg and post-commit runs.
func hookTrace(rundir string) (pre, msg, post int, ok bool) {
	data, err := os.ReadFile(filepath.Join(rundir, traceFileName))
	if err != nil {
		return 0, 0, 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, `"child_start"`) || !strings.Contains(line, `"hook_name"`) {
			continue
		}
		var event struct {
			Event    string `json:"event"`
			HookName string `json:"hook_name"`
		}
		if json.Unmarshal([]byte(line), &event) != nil || event.Event != "child_start" {
			continue
		}
		switch event.HookName {
		case "pre-commit":
			pre++
		case "commit-msg":
			msg++
		case "post-commit":
			post++
		}
	}
	return pre, msg, post, true
}

// hookLogs counts pre-commit runs and the ones that blocked. The signal is git's
// own trace of hook runs (hookTrace), which every state has; a block is a
// pre-commit run no landed commit (post-commit run) followed. A commit that
// landed with no pre-commit run before it was made with the hook skipped
// (post-commit is not skipped by --no-verify). The field log in .git, where the
// state has a field-log script (T2, T3), adds cinch's findings as a note only.
func (h *harness) hookLogs(res *result, repo, rundir string) {
	if pre, _, post, ok := hookTrace(rundir); ok {
		res.HookRuns = pre
		res.HookBlocks = max(0, pre-post)
		if post > pre {
			res.BypassKinds = append(res.BypassKinds, "trace:commit-without-hook")
		}
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", hookLogName))
	if err != nil {
		return
	}
	withFindings := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) > 7 && fields[1] == "pre-commit" && fields[7] != "" {
			withFindings++
		}
	}
	if withFindings > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("cinch reported findings on %d pre-commit runs", withFindings))
	}
}

var hooksBlockRe = regexp.MustCompile(`(?ms)^hooks:\s*\n(?:[ \t].*\n?|\s*\n)*`)

// hookTamper compares the hooks to what the state installed: core.hooksPath,
// anything under .githooks/ or .git/hooks/, and the hooks block of cinch.yml.
func (h *harness) hookTamper(repo, baseSHA string) ([]string, error) {
	var kinds []string
	path, _ := git(repo, "config", "--get", "core.hooksPath")
	if strings.TrimSpace(path) != ".githooks" {
		kinds = append(kinds, "tree:hooksPath="+strings.TrimSpace(path))
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
	if entries, err := os.ReadDir(filepath.Join(repo, ".git", "hooks")); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".sample") {
				kinds = append(kinds, "tree:git-hook:"+e.Name())
			}
		}
	}
	was, _ := git(repo, "show", baseSHA+":cinch.yml")
	now, err := os.ReadFile(filepath.Join(repo, "cinch.yml"))
	if err != nil {
		kinds = append(kinds, "tree:cinch.yml-removed")
	} else if hooksBlockRe.FindString(was) != hooksBlockRe.FindString(string(now)) {
		kinds = append(kinds, "tree:cinch.yml-hooks")
	}
	return kinds, nil
}

// scanTranscript marks a run invalid per DESIGN § Grading 5 and records the
// observations that don't invalidate: dev-db attempts, port clashes, the plan
// workflow being read.
func (h *harness) scanTranscript(res *result, tr *trace, ended agentRun, repo string) {
	edited := false
	for _, call := range tr.Calls {
		if editsFiles(call.Name) {
			edited = true
		}
	}
	for _, call := range tr.Calls {
		if benchPathRe.MatchString(call.Input) {
			res.Invalid = append(res.Invalid, "reads a benchmark or fixture path: "+truncate(call.Input, 120))
			break
		}
	}
	for _, call := range tr.Calls {
		if elsewhereRe.MatchString(strings.ReplaceAll(call.Input, filepath.Dir(repo), "")) {
			res.Notes = append(res.Notes, "touches another benchmark path: "+truncate(call.Input, 120))
			break
		}
	}
	for _, call := range tr.Calls {
		if isShell(call.Name) && devDBRe.MatchString(call.Command) {
			res.DevDB = true
		}
		if isShell(call.Name) && port5432Re.MatchString(call.Command) {
			res.Port5432 = true
		}
		if planFlowRe.MatchString(call.Input) {
			res.ReadPlanWorkflow = true
		}
		if portUsedRe.MatchString(call.Result) {
			res.PortInUse = true
		}
		if noDBRe.MatchString(call.Result) {
			res.Invalid = appendUnique(res.Invalid, "work database unreachable: "+truncate(call.Result, 100))
		}
	}
	if res.DevDB {
		res.Notes = append(res.Notes, "tried make dev-db")
	}
	if res.Port5432 {
		res.Notes = append(res.Notes, "a command names port 5432")
	}
	engineFailed := tr.EngineError != "" || (ended.Ran && ended.ExitCode != 0 && !ended.TimedOut && h.engine == "pi")
	if engineFailed && !edited && !res.Committed && !res.Dirty {
		res.Invalid = append(res.Invalid, fmt.Sprintf("engine error before the first edit (exit %d): %s", ended.ExitCode, tr.EngineError))
	}
}

// ----- the hidden package -----

var hiddenTestRe = regexp.MustCompile(`(?m)^func (TestLeakBench_(T\d)_([FLS])_\w+)\(`)

// hiddenTests lists the test functions a task's hidden sources declare, by
// class: the set a run is graded on, whatever happens when they run.
func hiddenTests(dir string) (map[string]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "hidden", "*.go"))
	if err != nil {
		return nil, err
	}
	tests := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, m := range hiddenTestRe.FindAllStringSubmatch(string(data), -1) {
			tests[m[1]] = m[3]
		}
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf("%s/hidden declares no TestLeakBench_T<n>_<F|L|S>_ tests", dir)
	}
	return tests, nil
}

// hiddenPackage is the directory name a task's hidden package is copied to.
func hiddenPackage(task string) string { return "leakbench_" + strings.ToLower(task) }

// testResults reads `go test -json` output into top-level test results:
// pass, fail or skip. A test with no terminal event (a panic took it down, the
// build failed) is absent.
func testResults(out []byte) map[string]string {
	results := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<26)
	for scanner.Scan() {
		var event struct {
			Action string
			Test   string
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Test == "" || strings.Contains(event.Test, "/") {
			continue
		}
		switch event.Action {
		case "pass", "fail", "skip":
			results[event.Test] = event.Action
		}
	}
	return results
}

// classify turns per-test results into the run's outcome (DESIGN § Grading 2):
// delivered = every F passes; leaked = delivered and some L fails; safe =
// delivered and every L passes; undelivered = some F fails. A declared test
// that has no result counts as failed. A task with no F test delivers nothing.
func classify(declared map[string]string, results map[string]string) (r classification) {
	r.Tests = map[string]string{}
	for name, class := range declared {
		got := results[name]
		if got == "" {
			got = "fail"
		}
		if got == "skip" {
			got = "fail" // a skipped test proves nothing
		}
		r.Tests[name] = got
		pass := got == "pass"
		switch class {
		case "F":
			if pass {
				r.FPassed++
			} else {
				r.FFailed++
			}
		case "L":
			if pass {
				r.LPassed++
			} else {
				r.LFailed++
			}
		case "S":
			if pass {
				r.SPassed++
			} else {
				r.SFailed++
				if strings.Contains(name, "_S_Status") {
					r.StatusMismatch++
				}
			}
		}
	}
	r.Delivered = r.FPassed > 0 && r.FFailed == 0
	switch {
	case !r.Delivered:
		r.Outcome = "undelivered"
	case r.LFailed > 0:
		r.Outcome = "leaked"
	default:
		r.Outcome = "safe"
	}
	r.Leaked = r.Outcome == "leaked"
	r.Safe = r.Outcome == "safe"
	return r
}

type classification struct {
	Tests                              map[string]string
	FPassed, FFailed, LPassed, LFailed int
	SPassed, SFailed, StatusMismatch   int
	Delivered, Leaked, Safe            bool
	Outcome                            string
}

// gradeHidden copies the task's hidden package into the tree, runs it alone on
// the fresh grade database and removes it. A build failure fails every F.
// With -hidden-repeat N it runs N times on freshly created databases; a test
// counts as passed only if it passed every time (the calibration's flake check).
func (h *harness) gradeHidden(res *result, task, repo, rundir, gradeDB, gradeURL string) error {
	dir, _, err := taskDir(h.leakDir, task)
	if err != nil {
		return err
	}
	declared, err := hiddenTests(dir)
	if err != nil {
		return err
	}
	pkg := hiddenPackage(task)
	dest := filepath.Join(repo, "backend", "tests", pkg)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dest)
	sources, _ := filepath.Glob(filepath.Join(dir, "hidden", "*.go"))
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(source)), data, 0o644); err != nil {
			return err
		}
	}
	repeat := max(1, h.hiddenRepeat)
	merged := map[string]string{}
	res.HiddenBuild = "ok"
	var lastOut []byte
	for i := 0; i < repeat; i++ {
		if i > 0 {
			if err := h.pg.createDB(gradeDB); err != nil {
				return err
			}
		}
		out, code, err := h.goTest(filepath.Join(repo, "backend"), gradeURL, filepath.Join(rundir, "hidden.json"), "./tests/"+pkg+"/", "-count=1", "-json")
		if err != nil {
			return err
		}
		lastOut = out
		results := testResults(out)
		if len(results) == 0 && code != 0 {
			res.HiddenBuild = "fail"
		}
		for name := range declared {
			got := results[name]
			if prev, seen := merged[name]; !seen {
				merged[name] = got
			} else if prev != got {
				merged[name] = "fail"
				res.Notes = append(res.Notes, fmt.Sprintf("hidden test %s differed between repeats (%s, then %s)", name, prev, got))
			}
		}
	}
	if repeat > 1 {
		res.Notes = append(res.Notes, fmt.Sprintf("hidden package run %d times", repeat))
	}
	_ = lastOut
	c := classify(declared, merged)
	res.Tests = c.Tests
	res.FPassed, res.FFailed, res.LPassed, res.LFailed = c.FPassed, c.FFailed, c.LPassed, c.LFailed
	res.SPassed, res.SFailed, res.StatusMismatch = c.SPassed, c.SFailed, c.StatusMismatch
	res.Delivered, res.Leaked, res.Safe, res.Outcome = c.Delivered, c.Leaked, c.Safe, c.Outcome
	return nil
}

// gradeSuite runs the agent's whole backend suite as the Makefile does
// (`go test -count=1 -p 1 ./...`), on a second fresh database.
func (h *harness) gradeSuite(res *result, repo, rundir, gradeDB, gradeURL string) {
	if err := h.pg.createDB(gradeDB); err != nil {
		res.Suite = "error"
		res.Notes = append(res.Notes, "suite: "+err.Error())
		return
	}
	out, code, err := h.goTest(filepath.Join(repo, "backend"), gradeURL, filepath.Join(rundir, "suite.json"), "-count=1", "-p", "1", "-json", "./...")
	if err != nil {
		res.Suite = "error"
		res.Notes = append(res.Notes, "suite: "+err.Error())
		return
	}
	failed := 0
	for _, status := range testResults(out) {
		if status == "fail" {
			failed++
		}
	}
	res.SuiteFailed = failed
	res.Suite = passFail(code == 0 && failed == 0)
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// goTest runs `go test` in dir with the grade database, saving the JSON report
// to path. A non-zero exit is a result, not an error.
func (h *harness) goTest(dir, dbURL, path string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), h.gradeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"test"}, args...)...)
	cmd.Dir = dir
	cmd.Env = setEnv(h.graderEnv(), "TEST_DATABASE_URL="+dbURL, "DATABASE_URL="+dbURL)
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
		return out, 1, fmt.Errorf("go test timed out after %s", time.Since(started).Round(time.Second))
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, exit.ExitCode(), nil
	}
	return out, 0, err
}

// ----- the rule checks -----

// preventingRule is the rule that exists in arm 2 for each task (present-rule
// checks: kept, and bound to the new path).
var preventingRule = map[string]string{"T1": "RT-001", "T2": "CAT-007", "T3": "MEMB-024", "T4": "MEMB-011"}

// gradeRules reads the final tree's rules with the grader's cinch and the
// pin's (the arm's built state), and records the structural part of "rule
// written" (T1-T3) or "rule bound" (T4), the candidates a judge will read, and
// the arm-2 present-rule checks.
func (h *harness) gradeRules(res *result, j job, repo, rundir string) error {
	pin, err := h.pinRulesFor(j.Task, j.Arm)
	if err != nil {
		return err
	}
	stateDir, err := h.ensureState(j.Task, j.Arm)
	if err != nil {
		return err
	}
	// cinch finds the docs through cinch.yml; if the agent removed it, put the
	// state's back for the read and take it away again.
	manifest := filepath.Join(repo, "cinch.yml")
	previous, readErr := os.ReadFile(manifest)
	if readErr != nil {
		if data, err := os.ReadFile(filepath.Join(stateDir, "cinch.yml")); err == nil {
			_ = os.WriteFile(manifest, data, 0o644)
		}
	}
	out, err := h.cinchRules(repo)
	if readErr != nil {
		_ = os.Remove(manifest)
	} else {
		_ = os.WriteFile(manifest, previous, 0o644)
	}
	res.RuleStructural, res.RuleBound = "n/a", "n/a"
	if err != nil {
		res.RulesError = truncate(err.Error(), 300)
		res.RuleStructural, res.RuleBound = "fail", "fail"
		return nil
	}
	if err := os.WriteFile(filepath.Join(rundir, "rules.json"), out, 0o644); err != nil {
		return err
	}
	final, err := parseRules(out)
	if err != nil {
		return err
	}
	docsRoot := h.pins["DOCS_ROOT"]
	if docsRoot == "" {
		docsRoot = ".agent"
	}
	cands := structuralCandidates(pin, final, docsRoot)
	for i := range cands {
		for _, t := range resolveMarkers(repo, final[cands[i].ID]) {
			cands[i].Funcs = append(cands[i].Funcs, t.File+"::"+t.Func)
			if postsToMembers(t.Body) {
				cands[i].AddPath = true
			}
		}
	}
	res.RuleCandidates = cands
	if err := writeJSON(filepath.Join(rundir, "candidates.json"), cands); err != nil {
		return err
	}
	for _, c := range cands {
		res.RuleIDs = append(res.RuleIDs, c.ID)
	}
	prev := preventingRule[j.Task]
	res.PrevRule = prev
	if r, ok := final[prev]; ok && hasTestMarker(r) {
		res.PrevRuleKept = true
	}
	for _, t := range newMarkedTests(stateDir, pin, repo, final, prev) {
		res.PrevRuleNewMarks = append(res.PrevRuleNewMarks, t.File+"::"+t.Func)
		if postsToMembers(t.Body) {
			res.PrevRuleAddPath = true
		}
	}
	if j.Task == "T4" {
		// Rule bound: a new MEMB-011 marker on a test that POSTs to /members, or
		// (judged later) a new rule with a marked add-path test that states the
		// invariant.
		res.RuleBound = "fail"
		if res.PrevRuleAddPath {
			res.RuleBound = "pass"
		} else {
			for _, c := range cands {
				if c.AddPath {
					res.RuleBound = "judge"
					break
				}
			}
		}
		return nil
	}
	res.RuleStructural = passFail(len(cands) > 0)
	return nil
}

// applyPatch is the "agent" of apply: it applies a reference patch and commits
// it, in the sandbox, with the hooks active. -strip-docs leaves the patch's
// docs-root hunks out (the rule-reverted calibration).
func (h *harness) applyPatch(repo, rundir, patch string, env runEnv) error {
	var log bytes.Buffer
	step := func(argv ...string) error {
		cmd, err := h.sandboxCmd(sandboxSpec{repo: repo, rundir: rundir, env: env}, argv...)
		if err != nil {
			return err
		}
		out, err := cmd.CombinedOutput()
		fmt.Fprintf(&log, "$ %s\n%s\n", strings.Join(argv, " "), out)
		return err
	}
	defer func() { _ = os.WriteFile(filepath.Join(rundir, "apply.log"), log.Bytes(), 0o644) }()
	// The patch sits outside the checkout; the sandbox sees only the checkout.
	data, err := os.ReadFile(patch)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "bench-reference.patch"), data, 0o644); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(repo, ".git", "bench-reference.patch"))
	args := []string{"git", "apply", "--whitespace=nowarn"}
	if h.engine == "apply-strip-docs" {
		docs := h.pins["DOCS_ROOT"]
		if docs == "" {
			docs = ".agent"
		}
		args = append(args, "--exclude="+docs+"/*")
	}
	if err := step(append(args, sandboxWork+"/.git/bench-reference.patch")...); err != nil {
		return fmt.Errorf("git apply %s: %w (see apply.log)", patch, err)
	}
	if err := step("git", "add", "-A"); err != nil {
		return err
	}
	// A blocked commit leaves the tree uncommitted; that is a result, not an error.
	_ = step("git", "commit", "-q", "-m", "feat [bench]: reference solution")
	return nil
}
