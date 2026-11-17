// Command leak runs the leak replay (plan 24, item D of plan 29): for each task,
// arm and run index an agent builds a feature in a pinned DeltaDocs state whose
// corpus has (arm 2) or lacks (arms 1 and 3) the rule that prevents a real
// authorization bug, and a mechanical grader says whether the feature works,
// whether the boundary held (hidden tests against the final tree), and whether
// the agent wrote the missing rule. The design is bench/leak/DESIGN.md; the
// builder brief is BUILD.md; the RUN section of README.md says how to use it.
//
// Usage: go run ./bench/leak <command> [flags]
//
//	run         (default) -engine pi|claude|fake|none [-stage 1|2] [-pilot] [-tasks T1,T2] [-arms 1,3] ...
//	apply       -apply TASK=PATCH [-arms 1,2,3] [-strip-docs]: commit a reference patch as the agent and grade it
//	states      [-tasks] [-arms] [-pin] [-out DIR]: build the constructed states and print their tree hashes
//	judge       -engine E -rubric FILE -judge claude|pi|fake: judge the rule texts of finished runs
//	calibrate   -judge ... -rubric FILE: the rule judge's probes (bench/recover's calibrate, per task)
//	manifest    write bench/private-manifest.txt (sha256 of every file under private/)
//
// CSV columns of a run (one row per attempt, in schedule order) are documented
// at the top of csv.go.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type config struct {
	pinsPath, leakDir, fixture, work, workspaces, cinch, out string
	engine, provider, model, thinking, claudeModel           string
	maxTurns, stage, perStage, workers                       int
	pilotN1, pilotN3                                         int
	seed                                                     int64
	timeout, gradeTimeout                                    time.Duration
	taskFilter                                               []string
	armFilter                                                []int
	apply                                                    map[string]string
	piNode, piCLI, fakeScript, smokePrompt                   string
	dryRun, rerunInvalid, keepWork, keepDBs, stripDocs       bool
	pilot, allowUnpinned, noSuite, noFrontendDeps            bool
	hiddenRepeat                                             int
	label                                                    string
}

// harness is the state every run shares.
type harness struct {
	config
	pins        map[string]string
	pg          pgConfig
	cinchBin    string
	sandbox     sandboxPaths
	baseEnv     []string
	runsDir     string
	csvMutex    sync.Mutex
	sizes       sizes
	shuffle     []job
	stateMu     sync.Mutex
	states      map[string]string // state key -> built directory
	stateHashes map[string]string // state key -> tree sha256
	pinRules    map[string]map[string]corpusRule
	pinRulesMu  sync.Mutex
	nodeMu      sync.Mutex
	nodeDirs    map[string]string
	judgeMap    map[string]map[string]judged // run id -> candidate id -> verdict (read by the CSV)
}

func main() {
	args := os.Args[1:]
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "run":
		err = runCommand(args, false)
	case "apply":
		err = runCommand(args, true)
	case "states":
		err = statesMain(args)
	case "judge":
		err = judgeMain(args)
	case "calibrate":
		err = calibrateMain(args)
	case "manifest":
		err = manifestMain(args)
	case "help", "-h", "--help":
		fmt.Println(strings.TrimSpace(usage))
	default:
		err = fmt.Errorf("unknown command %q\n%s", cmd, usage)
	}
	if err != nil {
		fail(err)
	}
}

const usage = `commands: run (default), apply, states, judge, calibrate, manifest`

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func defaultWork() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cinch-leak")
	}
	return filepath.Join(os.TempDir(), "cinch-leak")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "leak:", err)
	os.Exit(1)
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}

// commonFlags registers the flags every command shares.
func commonFlags(fs *flag.FlagSet, c *config) {
	home, _ := os.UserHomeDir()
	fs.StringVar(&c.pinsPath, "pins", filepath.Join("bench", "leak", "PINS"), "pin of record")
	fs.StringVar(&c.leakDir, "leak-dir", filepath.Join("bench", "leak"), "bench/leak: tasks, arms, states, private")
	fs.StringVar(&c.fixture, "fixture", filepath.Join(home, "code", "project_deltadocs"), "the private DeltaDocs repo, read with git archive/show/ls-tree/rev-parse only")
	fs.StringVar(&c.work, "work", defaultWork(), "cache directory: built states, caches, per-run outputs")
	fs.StringVar(&c.cinch, "cinch", "", "the grader's cinch binary (default: mise's, else PATH's); its version must match CINCH_VERSION")
	fs.BoolVar(&c.allowUnpinned, "allow-unpinned", false, "build states whose tree hash PINS does not yet record (calibration before prereg/leak-pilot); the hash is logged")
}

func runCommand(args []string, applyMode bool) error {
	var c config
	var tasks, arms string
	var apply multiFlag
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	commonFlags(fs, &c)
	fs.StringVar(&c.workspaces, "ws", "", "where agents' checkouts live (default <work>/ws)")
	fs.StringVar(&c.out, "o", "", "CSV output (default bench/leak/results-<engine>.csv; pilots bench/leak/pilot/results-<engine>.csv; T4 rows go to bench/leak/private/)")
	fs.StringVar(&c.engine, "engine", "", "pi, claude, fake (a script stands in), or none (the agent does nothing)")
	fs.StringVar(&c.provider, "provider", "", "pi provider (default PI_PROVIDER)")
	fs.StringVar(&c.model, "model", "", "pi model (default PI_MODEL)")
	fs.StringVar(&c.thinking, "thinking", "", "pi thinking level (default PI_THINKING)")
	fs.StringVar(&c.claudeModel, "claude-model", "", "claude model (default CLAUDE_MODEL)")
	fs.IntVar(&c.maxTurns, "max-turns", 0, "claude --max-turns (default CLAUDE_MAX_TURNS)")
	fs.DurationVar(&c.timeout, "timeout", 0, "wall clock per agent run, overriding the task's cap (default CAP_<TASK>; PILOT_CAP_MINUTES in a pilot)")
	fs.DurationVar(&c.gradeTimeout, "grade-timeout", 20*time.Minute, "wall clock per test run in the grader")
	fs.Int64Var(&c.seed, "seed", 0, "schedule seed (default SCHEDULE_SEED)")
	fs.IntVar(&c.stage, "stage", 1, "1 runs the first N_STAGE1_ARM13 indices of arms 1 and 3 (and arm 2's N_ARM2); 2 runs the rest of arms 1 and 3")
	fs.IntVar(&c.perStage, "n", 0, "runs per task in arms 1 and 3 in stage 1 (default N_STAGE1_ARM13); stage 2 is the rest up to N_TOTAL_ARM13")
	fs.IntVar(&c.workers, "workers", 0, "parallel runs (claude only; pi, fake and none are serial)")
	fs.IntVar(&c.pilotN1, "pilot-arm1", -1, "with -pilot: arm 1 runs per task (default 3 for pi, 1 for claude)")
	fs.IntVar(&c.pilotN3, "pilot-arm3", -1, "with -pilot: arm 3 runs per task (default 1 for pi, 0 for claude)")
	fs.StringVar(&tasks, "tasks", "", "comma-separated tasks (default all four)")
	fs.StringVar(&arms, "arms", "", "comma-separated arms (default each task's)")
	fs.Var(&apply, "apply", "TASK=PATCH: commit this patch as the agent and grade it (repeatable; implies engine apply)")
	fs.StringVar(&c.piNode, "pi-node", "", "node binary that runs pi (default: mise's node@latest)")
	fs.StringVar(&c.piCLI, "pi-cli", "", "pi's cli.js (default: the newest under ~/.npm/_npx)")
	fs.StringVar(&c.fakeScript, "fake-script", "", "with -engine fake: a bash script run in the sandbox in place of a model")
	fs.StringVar(&c.smokePrompt, "smoke-prompt", "", "with -engine pi or claude: send this file in place of each task's prompt (an infrastructure check, not a run); results go to their own dir")
	fs.BoolVar(&c.stripDocs, "strip-docs", false, "with -apply: leave the patch's docs-root hunks out (the rule-reverted calibration); results go to their own dir")
	fs.BoolVar(&c.pilot, "pilot", false, "run DESIGN's pilot schedule (per task: arm 1 x3, arm 3 x1 for pi) with PILOT_CAP_MINUTES; results go to their own dir and are excluded from every result")
	fs.BoolVar(&c.dryRun, "dry-run", false, "print the schedule and exit")
	fs.BoolVar(&c.rerunInvalid, "rerun-invalid", false, "rerun, once, runs whose first attempt was invalid; the invalid attempt is kept")
	fs.BoolVar(&c.keepWork, "keep-work", false, "keep each agent checkout after grading")
	fs.BoolVar(&c.keepDBs, "keep-dbs", false, "keep each run's databases after grading")
	fs.BoolVar(&c.noSuite, "no-suite", false, "skip the backend suite on a second fresh database (secondary)")
	fs.BoolVar(&c.noFrontendDeps, "no-frontend-deps", false, "do not install frontend/node_modules for the sandbox (faster calibration)")
	fs.StringVar(&c.label, "label", "", "suffix for the run's results directory and CSV, so calibration invocations (naive, reference, ...) keep their results apart")
	fs.IntVar(&c.hiddenRepeat, "hidden-repeat", 1, "run the hidden package N times on each tree; every run must pass for the tests to count (the flake check, 20 in calibration)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var err error
	if c.taskFilter = splitList(tasks); len(c.taskFilter) == 0 {
		c.taskFilter = nil
	}
	if c.armFilter, err = parseArms(arms); err != nil {
		return err
	}
	if applyMode || len(apply) > 0 {
		c.apply = map[string]string{}
		for _, spec := range apply {
			task, path, ok := strings.Cut(spec, "=")
			if !ok {
				return fmt.Errorf("-apply wants TASK=PATCH, got %q", spec)
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			c.apply[task] = abs
			c.taskFilter = appendUnique(c.taskFilter, task)
		}
		if len(c.apply) == 0 {
			return fmt.Errorf("apply wants -apply TASK=PATCH")
		}
		c.engine = "apply"
		if c.stripDocs {
			c.engine = "apply-strip-docs"
		}
	}
	return run(c)
}

func parseArms(s string) ([]int, error) {
	var out []int
	for _, part := range splitList(s) {
		arm, err := strconv.Atoi(part)
		if err != nil || arm < 1 || arm > 3 {
			return nil, fmt.Errorf("-arms wants numbers from 1 to 3, got %q", part)
		}
		out = append(out, arm)
	}
	return out, nil
}

// calibration engines run once per task and arm, outside the stages: no model
// runs, so a stage's counts do not apply.
func calibration(engine string) bool {
	return engine == "none" || engine == "apply" || engine == "apply-strip-docs" || engine == "fake"
}

func newHarness(c config) (*harness, error) {
	pins, err := loadPins(c.pinsPath)
	if err != nil {
		return nil, err
	}
	h := &harness{config: c, pins: pins, states: map[string]string{}, stateHashes: map[string]string{},
		pinRules: map[string]map[string]corpusRule{}, nodeDirs: map[string]string{}, judgeMap: map[string]map[string]judged{}}
	h.fixture = expandHome(h.fixture)
	h.work = expandHome(h.work)
	if h.provider == "" {
		h.provider = pins["PI_PROVIDER"]
	}
	if h.model == "" {
		h.model = pins["PI_MODEL"]
	}
	if h.thinking == "" {
		h.thinking = pins["PI_THINKING"]
	}
	if h.claudeModel == "" {
		h.claudeModel = pins["CLAUDE_MODEL"]
	}
	if h.maxTurns == 0 {
		h.maxTurns = pinInt(pins, "CLAUDE_MAX_TURNS", 80)
	}
	if h.seed == 0 {
		h.seed, _ = strconv.ParseInt(pins["SCHEDULE_SEED"], 10, 64)
	}
	if h.workers == 0 {
		h.workers = 1
		if h.engine == "claude" {
			h.workers = 3
		}
	}
	if h.engine != "claude" {
		h.workers = 1
	}
	if h.gradeTimeout == 0 {
		h.gradeTimeout = 20 * time.Minute
	}
	if h.workspaces == "" {
		h.workspaces = filepath.Join(h.work, "ws")
	}
	n1 := pinInt(pins, "N_STAGE1_ARM13", 8)
	if h.perStage > 0 {
		n1 = h.perStage
	}
	h.sizes = sizes{NStage1: n1, NTotal: pinInt(pins, "N_TOTAL_ARM13", 15), NArm2: pinInt(pins, "N_ARM2", 5)}
	if h.sizes.NStage1 > h.sizes.NTotal {
		h.sizes.NTotal = h.sizes.NStage1
	}
	h.pg = newPG(pins)
	return h, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// jobsFor is the selected, ordered job list of an invocation.
func (h *harness) jobsFor() []job {
	armsOfTask := func(task string) []int { return armsOf(h.pins, task) }
	switch {
	case h.pilot:
		n1, n3 := h.pilotN1, h.pilotN3
		if n1 < 0 {
			n1 = 3
			if h.engine == "claude" {
				n1 = 1
			}
		}
		if n3 < 0 {
			n3 = 1
			if h.engine == "claude" {
				n3 = 0
			}
		}
		jobs := pilotSchedule(h.seed, h.engine, h.taskFilter, n1, n3)
		if h.armFilter == nil {
			return jobs
		}
		var out []job
		for _, j := range jobs {
			if containsInt(h.armFilter, j.Arm) {
				out = append(out, j)
			}
		}
		return out
	case calibration(h.engine) || h.smokePrompt != "":
		// One run of each selected task and arm, in canonical order.
		var out []job
		for _, task := range canonicalTasks {
			if h.taskFilter != nil && !contains(h.taskFilter, task) {
				continue
			}
			for _, arm := range armsOfTask(task) {
				if h.armFilter != nil && !containsInt(h.armFilter, arm) {
					continue
				}
				out = append(out, job{Position: len(out), Task: task, Arm: arm})
			}
		}
		return out
	}
	h.shuffle = schedule(h.seed, h.engine, armsOfTask, h.sizes)
	return selectJobs(h.shuffle, h.stage, h.sizes, h.taskFilter, h.armFilter)
}

func (c config) runName() string {
	name := c.engine
	if c.label != "" && strings.HasPrefix(name, "apply") {
		name = "apply" // -strip-docs and plain apply share a name once labelled
	}
	if c.smokePrompt != "" {
		name = "smoke-" + c.engine
	}
	if c.pilot {
		name = "pilot-" + name
	}
	if c.label != "" {
		name += "-" + c.label
	}
	return name
}

// capFor is the wall clock for a task's runs: -timeout, else the pilot cap in a
// pilot, else the task's registered cap.
func (h *harness) capFor(j job) (time.Duration, error) {
	if h.timeout > 0 {
		return h.timeout, nil
	}
	key := "CAP_" + j.Task
	if j.Pilot {
		key = "PILOT_CAP_MINUTES"
	}
	if pinTODO(h.pins, key) {
		if calibration(h.engine) {
			return 30 * time.Minute, nil
		}
		return 0, fmt.Errorf("PINS has no %s (TODO): set at prereg/leak by the pilot rule, or pass -timeout", key)
	}
	minutes, err := strconv.Atoi(h.pins[key])
	if err != nil || minutes <= 0 {
		return 0, fmt.Errorf("PINS %s=%q is not a number of minutes", key, h.pins[key])
	}
	return time.Duration(minutes) * time.Minute, nil
}

func run(c config) error {
	h, err := newHarness(c)
	if err != nil {
		return err
	}
	if h.engine == "" {
		return fmt.Errorf("-engine wants pi, claude, fake or none")
	}
	if !contains([]string{"pi", "claude", "fake", "none", "apply", "apply-strip-docs"}, h.engine) {
		return fmt.Errorf("unknown engine %q", h.engine)
	}
	if h.stage != 1 && h.stage != 2 {
		return fmt.Errorf("-stage wants 1 or 2")
	}
	if h.smokePrompt != "" && h.engine != "pi" && h.engine != "claude" {
		return fmt.Errorf("-smoke-prompt wants -engine pi or claude")
	}
	jobs := h.jobsFor()
	if h.dryRun {
		fmt.Printf("# engine %s, seed %d, %s, %d runs\n", h.engine, h.seed, h.scheduleLabel(), len(jobs))
		for i, j := range jobs {
			label := j.id()
			if j.Pilot {
				label = "pilot " + label
			}
			fmt.Printf("%3d  pos %3d  %-22s task %s  arm %d  index %d\n", i+1, j.Position, label, j.Task, j.Arm, j.Index)
		}
		return nil
	}
	if len(jobs) == 0 {
		return fmt.Errorf("no runs selected")
	}
	if err := h.setup(jobs); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	h.runsDir = filepath.Join(h.work, "runs", h.runName())
	if err := os.MkdirAll(h.runsDir, 0o755); err != nil {
		return err
	}
	if err := h.writeMeta(jobs); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = h.runAll(ctx, jobs)
	if werr := h.writeCSV(); werr != nil && err == nil {
		err = werr
	}
	return err
}

func (h *harness) scheduleLabel() string {
	switch {
	case h.pilot:
		return "pilot"
	case calibration(h.engine) || h.smokePrompt != "":
		return "one run per task and arm"
	}
	return fmt.Sprintf("stage %d", h.stage)
}

// setup verifies everything a real run depends on before any of it starts.
func (h *harness) setup(jobs []job) error {
	h.baseEnv = cleanEnv(os.Environ())
	var tasks []string
	for _, j := range jobs {
		tasks = appendUnique(tasks, j.Task)
	}
	pinKeys := []string{"CINCH_VERSION", "SCHEDULE_SEED"}
	for _, task := range tasks {
		pinKeys = append(pinKeys, task+"_BASE_SHA")
	}
	if !calibration(h.engine) {
		if h.pilot {
			pinKeys = append(pinKeys, "PILOT_CAP_MINUTES")
		}
		if h.engine == "claude" {
			pinKeys = append(pinKeys, "CLAUDE_VERSION")
		}
	}
	if err := requirePins(h.pins, pinKeys...); err != nil {
		return err
	}
	if err := h.verifyPrivate(tasks); err != nil {
		return err
	}
	var err error
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
	if err := h.resolveSHAs(tasks); err != nil {
		return err
	}
	if err := h.resolveToolchain(); err != nil {
		return err
	}
	if err := h.resolveEngine(); err != nil {
		return err
	}
	if err := h.verifySandbox(); err != nil {
		return err
	}
	if err := h.pg.ensureRunning(); err != nil {
		return err
	}
	for _, j := range jobs {
		if _, err := h.capFor(j); err != nil {
			return err
		}
		if _, _, err := taskDir(h.leakDir, j.Task); err != nil {
			return err
		}
	}
	return h.preflightTasks(jobs)
}

// preflightTasks fails early when a scheduled task lacks what a run needs,
// rather than after an agent has already spent its cap.
func (h *harness) preflightTasks(jobs []job) error {
	seen := map[string]bool{}
	var missing []string
	for _, j := range jobs {
		if seen[j.Task] {
			continue
		}
		seen[j.Task] = true
		dir, _, err := taskDir(h.leakDir, j.Task)
		if err != nil {
			return err
		}
		needs := []string{"prompt.md"}
		if calibration(h.engine) && h.smokePrompt == "" {
			needs = nil
		}
		for _, name := range needs {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				missing = append(missing, filepath.Join(dir, name))
			}
		}
		if hidden, _ := filepath.Glob(filepath.Join(dir, "hidden", "*.go")); len(hidden) == 0 {
			missing = append(missing, filepath.Join(dir, "hidden", "*.go"))
		}
		if patch, ok := h.apply[j.Task]; ok {
			if _, err := os.Stat(patch); err != nil {
				missing = append(missing, patch)
			}
		}
	}
	if h.engine == "apply" || h.engine == "apply-strip-docs" {
		for task := range h.apply {
			if !seen[task] {
				missing = append(missing, "task "+task+" is not scheduled")
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("tasks are incomplete:\n  %s", strings.Join(missing, "\n  "))
	}
	return nil
}

func cleanEnv(env []string) []string {
	var out []string
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !contains([]string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_CHILD_SESSION"}, name) {
			out = append(out, entry)
		}
	}
	return out
}

// graderEnv is the grader's environment: the ordinary one with the harness's
// Go build cache, so the sandbox's overlay starts warm.
func (h *harness) graderEnv() []string {
	env := h.baseEnv
	if h.sandbox.goBuildCache != "" {
		env = setEnv(env, "GOCACHE="+h.sandbox.goBuildCache)
	}
	return env
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

// findCinch picks the grader's own cinch. A mise shim is not a binary, so
// mise is asked where the real one is.
func findCinch(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
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

// writeMeta records everything that identifies this invocation next to its
// runs, so a result can be traced to the seed, pin and tool versions.
func (h *harness) writeMeta(jobs []job) error {
	meta := map[string]any{
		"engine": h.engine, "seed": h.seed, "stage": h.stage, "pilot": h.pilot, "sizes": h.sizes,
		"cinch": h.cinchBin, "cinch_version": h.pins["CINCH_VERSION"], "state_tree_sha256": h.stateHashes,
		"fixture_bases": map[string]string{"T1": h.pins["T1_BASE_SHA"], "T2": h.pins["T2_BASE_SHA"], "T3": h.pins["T3_BASE_SHA"]},
		"pi":            map[string]string{"provider": h.provider, "model": h.model, "thinking": h.thinking, "version": toolVersion("pi")},
		"claude":        map[string]any{"model": h.claudeModel, "max_turns": h.maxTurns, "version": toolVersion("claude")},
		"runs":          len(jobs), "started": time.Now().Format(time.RFC3339),
	}
	name := fmt.Sprintf("meta-stage%d.json", h.stage)
	if h.pilot {
		name = "meta-pilot.json"
	}
	return writeJSON(filepath.Join(h.runsDir, name), meta)
}

func toolVersion(name string) string {
	out, err := exec.Command(name, "--version").CombinedOutput()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(out))
}

func (h *harness) runAll(ctx context.Context, jobs []job) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	queue := make(chan job)
	var wg sync.WaitGroup
	var failure error
	var failureMutex sync.Mutex
	for w := 0; w < h.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				if err := h.runOne(ctx, j); err != nil {
					failureMutex.Lock()
					if failure == nil && ctx.Err() == nil {
						failure = fmt.Errorf("%s: %w", j.id(), err)
					}
					failureMutex.Unlock()
					cancel()
				}
			}
		}()
	}
	for i, j := range jobs {
		select {
		case queue <- j:
			logf("[%d/%d] %s queued", i+1, len(jobs), j.id())
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(queue)
	wg.Wait()
	if failure != nil {
		return failure
	}
	return ctx.Err()
}

// runOne runs one job unless it already has a result. An interrupted run left
// no result.json, so it is cleared and redone; a first attempt marked invalid
// is set aside and redone once under -rerun-invalid.
func (h *harness) runOne(ctx context.Context, j job) error {
	dir := filepath.Join(h.runsDir, j.id())
	attempt := 1
	if calibration(h.engine) || h.smokePrompt != "" {
		// Calibration runs are cheap and meant to be repeated: always fresh.
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if prior, err := readResult(dir); err == nil {
		if !(h.rerunInvalid && prior.Status == "invalid" && prior.Attempt == 1) {
			return nil
		}
		if err := os.Rename(dir, dir+".attempt1"); err != nil {
			return err
		}
		attempt = 2
	} else if _, err := os.Stat(dir + ".attempt1"); err == nil {
		attempt = 2
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	res, err := h.execute(ctx, j, dir, attempt)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "result.json"), res); err != nil {
		return err
	}
	logf("%s: %s outcome=%s rule-structural=%s committed=%t bypass=%t %s", j.id(), res.Status,
		res.Outcome, res.RuleStructural, res.Committed, res.Bypass, strings.Join(res.Invalid, "; "))
	return h.writeCSV()
}

func readResult(dir string) (result, error) {
	var res result
	data, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		return res, err
	}
	return res, json.Unmarshal(data, &res)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func git(dir string, args ...string) (string, error) {
	return gitEnv(dir, nil, args...)
}

func gitEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…"
}

func statesMain(args []string) error {
	var c config
	var tasks, arms, out string
	var pin bool
	fs := flag.NewFlagSet("states", flag.ExitOnError)
	commonFlags(fs, &c)
	fs.StringVar(&tasks, "tasks", "", "comma-separated tasks (default all with a state script)")
	fs.StringVar(&arms, "arms", "1,2", "state arms to build (arm 3 shares arm 1)")
	fs.StringVar(&out, "out", "", "copy each built state (a one-commit repository) under this directory")
	fs.BoolVar(&pin, "pin", false, "print PINS lines (<TASK>_ARM<k>_TREE_SHA256=...) instead of 'hash  state'")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.allowUnpinned = c.allowUnpinned || pin
	h, err := newHarness(c)
	if err != nil {
		return err
	}
	h.baseEnv = cleanEnv(os.Environ())
	if h.cinchBin, err = findCinch(h.cinch); err != nil {
		return err
	}
	if err := requirePins(h.pins, "CINCH_VERSION"); err != nil {
		return err
	}
	if version, err := exec.Command(h.cinchBin, "version").Output(); err != nil || strings.TrimSpace(string(version)) != h.pins["CINCH_VERSION"] {
		return fmt.Errorf("grader cinch %s is %q, but PINS records %s", h.cinchBin, strings.TrimSpace(string(version)), h.pins["CINCH_VERSION"])
	}
	taskList := splitList(tasks)
	if len(taskList) == 0 {
		taskList = canonicalTasks
	}
	if err := h.verifyPrivate(taskList); err != nil {
		return err
	}
	if err := h.resolveSHAs(taskList); err != nil {
		return err
	}
	armList, err := parseArms(arms)
	if err != nil {
		return err
	}
	return statesCommand(h, taskList, armList, out, pin)
}
