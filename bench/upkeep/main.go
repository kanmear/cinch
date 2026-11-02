// Command upkeep runs the upkeep ablation: for each task, arm and run index an
// agent changes a pinned Budgetron checkout, and a mechanical grader says
// whether the code behaves (hidden tests) and whether the rule corpus stayed
// true (cinch's own rule inventory against the task's key). The design and its
// grading key are pre-registered in bench/upkeep/DESIGN.md.
//
// Usage: go run ./bench/upkeep -engine pi|claude|none [-stage 1|2] [-workers N]
//
//	[-tasks S1,C2] [-arms 1,2,3] [-seed N] [-dry-run] [-rerun-invalid]
//	[-apply TASK=PATCH [-apply-strip-docs]] [-engine fake -fake-script FILE]
//	[-repo PATH] [-ref REF] [-cinch PATH] [-work DIR]
//
// The fixture repo is only ever fetched from. -apply commits a patch as the
// "agent" (arm hooks active) and grades it, which is how the key, the hidden
// tests and the arms are calibrated before a single model call;
// -apply-strip-docs leaves the patch's .docs hunks out, so the corpus must
// fail. -engine fake runs a bash script in the sandbox in place of a model.
//
// Every agent process (and every -apply step) runs in a bubblewrap sandbox,
// identical in all arms except for cinch: the checkout at /work, an empty
// home holding only what the engine needs, the toolchain, system dirs
// read-only, the network shared. Arm 3 alone has the grader's cinch, at
// /opt/cinch/bin; in arms 1 and 2 no directory holding a cinch is bound.
//
// Layout under -work: base-<sha>/ is the cached, generated checkout every run
// is copied from; runs/<engine>/<run>/ holds each run's result.json, transcript
// and logs; the agent's own checkout lives under -ws at an opaque path, so its
// cwd names neither the arm nor the task.
//
// CSV columns (one row per run attempt, ordered by schedule position):
//
//	run_id, engine, task, arm, index, attempt, position, seed: which run
//	base_sha, head_sha: the neutral commit and where the agent left HEAD
//	status: graded, or invalid (see invalid_reason; rerun once, never dropped)
//	timed_out, exit_code, wall_s, engine_error: how the agent process ended
//	turns, input_tokens, output_tokens, cache_read, cost_usd: usage, if reported
//	committed: HEAD moved past base; commits: how many; dirty: uncommitted changes remain
//	behavior: hidden tests pass|fail (a build failure is fail); hidden_passed, hidden_failed
//	corpus: pass|fail|n/a (arm 1 has no corpus); corpus_reasons
//	upkeep: behavior and corpus both pass; n/a in arm 1
//	bypass, bypass_kinds: --no-verify, commit -n, hooksPath or hook edits
//	hook_runs, hook_blocks: pre-commit runs and the ones that failed
//	rules_changed: rule texts that differ from the pin (arms 2 and 3), rules_changed_ids
//	suite, suite_failed: full `flutter test -x perf` on the final tree (secondary)
//	notes: non-invalidating observations
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	canonicalTasks = []string{"C1", "C2", "C3", "S1", "S2", "S3"}
	canonicalArms  = []int{1, 2, 3}
)

type config struct {
	repo, ref, pinsPath, work, workspaces, cinch, objectbox, flutter string
	tasksDir, armsDir, out, engine, stateDir                         string
	provider, model, thinking, claudeModel                           string
	maxTurns, stage, perStage, workers                               int
	seed                                                             int64
	timeout, gradeTimeout                                            time.Duration
	taskFilter                                                       []string
	armFilter                                                        []int
	apply                                                            map[string]string
	piNode, piCLI, fakeScript, smokePrompt, keysRef                  string
	dryRun, rerunInvalid, keepWork, stripDocs, regrade               bool
}

// job is one scheduled run. Position is its place in the full shuffle of both
// stages, so a run keeps its slot whichever stage selects it.
type job struct {
	Position int
	Task     string
	Arm      int
	Index    int
}

func (j job) id() string { return fmt.Sprintf("%s-arm%d-r%d", j.Task, j.Arm, j.Index) }

// result is what one run records in result.json and the CSV.
type result struct {
	RunID         string         `json:"run_id"`
	Engine        string         `json:"engine"`
	Task          string         `json:"task"`
	Arm           int            `json:"arm"`
	Index         int            `json:"index"`
	Attempt       int            `json:"attempt"`
	Position      int            `json:"position"`
	Seed          int64          `json:"seed"`
	BaseSHA       string         `json:"base_sha"`
	HeadSHA       string         `json:"head_sha"`
	Status        string         `json:"status"`
	Invalid       []string       `json:"invalid_reason,omitempty"`
	TimedOut      bool           `json:"timed_out"`
	ExitCode      int            `json:"exit_code"`
	WallMS        int64          `json:"wall_ms"`
	EngineError   string         `json:"engine_error,omitempty"`
	Turns         int            `json:"turns"`
	Input         int            `json:"input_tokens"`
	Output        int            `json:"output_tokens"`
	CacheRead     int            `json:"cache_read"`
	CacheWrite    int            `json:"cache_write"`
	CostUSD       float64        `json:"cost_usd"`
	Tools         map[string]int `json:"tools"`
	Committed     bool           `json:"committed"`
	Commits       int            `json:"commits"`
	Dirty         bool           `json:"dirty"`
	Behavior      string         `json:"behavior"`
	HiddenPassed  int            `json:"hidden_passed"`
	HiddenFailed  int            `json:"hidden_failed"`
	Corpus        string         `json:"corpus"`
	CorpusReasons []string       `json:"corpus_reasons,omitempty"`
	Upkeep        string         `json:"upkeep"`
	Bypass        bool           `json:"bypass"`
	BypassKinds   []string       `json:"bypass_kinds,omitempty"`
	HookRuns      int            `json:"hook_runs"`
	HookBlocks    int            `json:"hook_blocks"`
	RulesChanged  []string       `json:"rules_changed"`
	Suite         string         `json:"suite"`
	SuiteFailed   int            `json:"suite_failed"`
	Notes         []string       `json:"notes,omitempty"`
	CorpusFirst   string         `json:"corpus_first,omitempty"` // as first graded, kept by -regrade when a key amendment changed it
	UpkeepFirst   string         `json:"upkeep_first,omitempty"`
}

func main() {
	home, _ := os.UserHomeDir()
	var c config
	var tasks, arms string
	var apply multiFlag
	flag.StringVar(&c.pinsPath, "pins", filepath.Join("bench", "upkeep", "PINS"), "pin of record: fixture SHA, cinch version, engine settings, seed")
	flag.StringVar(&c.repo, "repo", filepath.Join(home, "code", "budgetron.scrubbed.git"), "fixture repo (the scrubbed bare repo), fetched from and never written to")
	flag.StringVar(&c.ref, "ref", "", "fixture ref (default FIXTURE_REF from PINS); must resolve to FIXTURE_SHA")
	flag.StringVar(&c.work, "work", defaultWork(), "cache directory for the base checkout and per-run outputs")
	flag.StringVar(&c.workspaces, "ws", "", "where agents' checkouts live (default <work>/ws)")
	flag.StringVar(&c.cinch, "cinch", "", "the grader's cinch binary, absolute path (default: mise's, else PATH's)")
	flag.StringVar(&c.objectbox, "objectbox", filepath.Join(home, "code", "budgetron", ".objectbox", "lib"), "directory holding libobjectbox.so and VERSION, copied into the base checkout")
	flag.StringVar(&c.flutter, "flutter", filepath.Join(home, "dev", "flutter"), "Flutter SDK root; its bin is on every arm's PATH")
	flag.StringVar(&c.tasksDir, "tasks-dir", filepath.Join("bench", "upkeep", "tasks"), "task directories (read only)")
	flag.StringVar(&c.armsDir, "arms-dir", filepath.Join("bench", "upkeep", "arms"), "arm AGENTS.md files")
	flag.StringVar(&c.out, "o", "", "CSV output (default bench/upkeep/results-<engine>.csv)")
	flag.StringVar(&c.engine, "engine", "", "pi, claude, fake (a script stands in), or none (the agent does nothing); apply is implied by -apply")
	flag.StringVar(&c.provider, "provider", "", "pi provider (default PI_PROVIDER)")
	flag.StringVar(&c.model, "model", "", "pi model (default PI_MODEL)")
	flag.StringVar(&c.thinking, "thinking", "", "pi thinking level (default PI_THINKING)")
	flag.StringVar(&c.claudeModel, "claude-model", "", "claude model (default CLAUDE_MODEL)")
	flag.IntVar(&c.maxTurns, "max-turns", 0, "claude --max-turns (default CLAUDE_MAX_TURNS)")
	flag.DurationVar(&c.timeout, "timeout", 0, "wall clock per agent run (default WALL_CLOCK_MINUTES)")
	flag.DurationVar(&c.gradeTimeout, "grade-timeout", 20*time.Minute, "wall clock per test-suite run in the grader")
	flag.Int64Var(&c.seed, "seed", 0, "schedule seed (default SCHEDULE_SEED)")
	flag.IntVar(&c.stage, "stage", 1, "1 runs indices 0..n-1, 2 runs n..2n-1, on the same shuffle")
	flag.IntVar(&c.perStage, "n", 5, "runs per task and arm in one stage")
	flag.IntVar(&c.workers, "workers", 0, "parallel runs (default 1 for pi, 3 for claude, 1 otherwise)")
	flag.StringVar(&tasks, "tasks", "", "comma-separated tasks to run (default all six)")
	flag.StringVar(&arms, "arms", "", "comma-separated arms to run (default 1,2,3)")
	flag.Var(&apply, "apply", "TASK=PATCH: commit this patch as the agent and grade it (repeatable)")
	flag.StringVar(&c.piNode, "pi-node", "", "node binary that runs pi (default: mise's node@latest)")
	flag.StringVar(&c.piCLI, "pi-cli", "", "pi's cli.js (default: the newest under ~/.npm/_npx)")
	flag.StringVar(&c.fakeScript, "fake-script", "", "with -engine fake: a bash script run in the sandbox in place of a model")
	flag.StringVar(&c.smokePrompt, "smoke-prompt", "", "with -engine pi or claude: send this file in place of each task's prompt (an infrastructure check, not a run); results go to their own dir")
	flag.BoolVar(&c.stripDocs, "apply-strip-docs", false, "with -apply: leave the patch's .docs hunks out in every arm (corpus-reverted calibration); results go to their own dir")
	flag.BoolVar(&c.regrade, "regrade", false, "re-evaluate the corpus key of every graded run from its saved rules.json; nothing reruns")
	flag.StringVar(&c.keysRef, "keys-ref", "", "with -regrade: take the keys from this git ref and write only the CSV (a sensitivity table)")
	flag.BoolVar(&c.dryRun, "dry-run", false, "print the schedule and exit")
	flag.BoolVar(&c.rerunInvalid, "rerun-invalid", false, "rerun, once, runs whose first attempt was invalid; the invalid attempt is kept")
	flag.BoolVar(&c.keepWork, "keep-work", false, "keep each agent checkout after grading")
	flag.Parse()

	var err error
	if c.taskFilter = splitList(tasks); len(c.taskFilter) == 0 {
		c.taskFilter = nil
	}
	if c.armFilter, err = parseArms(arms); err != nil {
		fail(err)
	}
	if len(apply) > 0 {
		c.apply = map[string]string{}
		for _, spec := range apply {
			task, path, ok := strings.Cut(spec, "=")
			if !ok {
				fail(fmt.Errorf("-apply wants TASK=PATCH, got %q", spec))
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				fail(err)
			}
			c.apply[task] = abs
			c.taskFilter = appendUnique(c.taskFilter, task)
		}
		if c.engine == "" {
			c.engine = "apply"
		}
		if c.stripDocs {
			c.engine = "apply-strip-docs"
		}
	}
	if err := run(c); err != nil {
		fail(err)
	}
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func defaultWork() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cinch-upkeep")
	}
	return filepath.Join(os.TempDir(), "cinch-upkeep")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "upkeep:", err)
	os.Exit(1)
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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

func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

// schedule is the full randomized order of every task, arm and run index of
// both stages for one engine. The shuffle is a Fisher-Yates over PCG output
// with the modulo taken by hand: the standard library documents neither
// Shuffle's algorithm nor IntN's as stable, and this order is pre-registered.
func schedule(seed int64, engine string, tasks []string, perStage int) []job {
	var jobs []job
	for _, task := range tasks {
		for _, arm := range canonicalArms {
			for index := 0; index < 2*perStage; index++ {
				jobs = append(jobs, job{Task: task, Arm: arm, Index: index})
			}
		}
	}
	hasher := fnv.New64a()
	hasher.Write([]byte(engine))
	source := rand.NewPCG(uint64(seed), hasher.Sum64())
	for i := len(jobs) - 1; i > 0; i-- {
		j := int(source.Uint64() % uint64(i+1))
		jobs[i], jobs[j] = jobs[j], jobs[i]
	}
	for i := range jobs {
		jobs[i].Position = i
	}
	return jobs
}

// selectJobs keeps one stage's indices and the requested tasks and arms, in
// shuffle order.
func selectJobs(all []job, stage, perStage int, tasks []string, arms []int) []job {
	var out []job
	for _, j := range all {
		if j.Index < (stage-1)*perStage || j.Index >= stage*perStage {
			continue
		}
		if tasks != nil && !contains(tasks, j.Task) {
			continue
		}
		if arms != nil && !containsInt(arms, j.Arm) {
			continue
		}
		out = append(out, j)
	}
	return out
}

func contains(list []string, item string) bool {
	for _, existing := range list {
		if existing == item {
			return true
		}
	}
	return false
}

func containsInt(list []int, item int) bool {
	for _, existing := range list {
		if existing == item {
			return true
		}
	}
	return false
}

// harness is the state every run shares.
type harness struct {
	config
	pins          map[string]string
	pinSHA        string
	baseDir       string
	cinchBin      string
	pinRules      map[string]corpusRule
	pinCheck      []byte // the pin's cinch.yml, restored for grading arms that stripped it
	commitPattern string
	sandbox       sandboxPaths
	baseEnv       []string
	runsDir       string
	csvMutex      sync.Mutex
	universe      []string
	shuffle       []job
}

func run(c config) error {
	pins, err := loadPins(c.pinsPath)
	if err != nil {
		return err
	}
	c = withPinDefaults(c, pins)
	if c.engine == "" {
		return fmt.Errorf("-engine wants pi, claude or none")
	}
	if !contains([]string{"pi", "claude", "fake", "none", "apply", "apply-strip-docs"}, c.engine) {
		return fmt.Errorf("unknown engine %q", c.engine)
	}
	if c.stage != 1 && c.stage != 2 {
		return fmt.Errorf("-stage wants 1 or 2")
	}
	if c.smokePrompt != "" && c.engine != "pi" && c.engine != "claude" {
		return fmt.Errorf("-smoke-prompt wants -engine pi or claude")
	}
	if calibration(c.engine) || c.smokePrompt != "" {
		c.perStage = 1 // a calibration run needs one of each, not a stage
	}

	h := &harness{config: c, pins: pins}
	h.universe = unionTasks(canonicalTasks, c.taskFilter)
	h.shuffle = schedule(c.seed, c.engine, h.universe, c.perStage)
	jobs := selectJobs(h.shuffle, c.stage, c.perStage, c.taskFilter, c.armFilter)

	if c.dryRun {
		fmt.Printf("# engine %s, seed %d, stage %d, %d runs\n", c.engine, c.seed, c.stage, len(jobs))
		for i, j := range jobs {
			fmt.Printf("%3d  pos %3d  %-9s task %s  arm %d  index %d\n", i+1, j.Position, j.id(), j.Task, j.Arm, j.Index)
		}
		return nil
	}

	if c.regrade {
		return h.regradeAll()
	}
	if c.keysRef != "" {
		return fmt.Errorf("-keys-ref wants -regrade")
	}

	if err := h.preflightTasks(jobs); err != nil {
		return err
	}
	if err := h.setup(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	h.runsDir = filepath.Join(c.work, "runs", c.runName())
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

// calibration engines run once per task and arm, outside the stages.
// runName names a run's results dir and CSV, keeping smoke runs apart.
func (c config) runName() string {
	if c.smokePrompt != "" {
		return "smoke-" + c.engine
	}
	return c.engine
}

func calibration(engine string) bool {
	return engine == "none" || engine == "apply" || engine == "apply-strip-docs" || engine == "fake"
}

func unionTasks(canonical, extra []string) []string {
	out := append([]string{}, canonical...)
	for _, task := range extra {
		out = appendUnique(out, task)
	}
	sort.Strings(out)
	return out
}

func withPinDefaults(c config, pins map[string]string) config {
	pickString := func(current *string, key string) {
		if *current == "" {
			*current = pins[key]
		}
	}
	pickString(&c.ref, "FIXTURE_REF")
	pickString(&c.provider, "PI_PROVIDER")
	pickString(&c.model, "PI_MODEL")
	pickString(&c.thinking, "PI_THINKING")
	pickString(&c.claudeModel, "CLAUDE_MODEL")
	if c.maxTurns == 0 {
		c.maxTurns, _ = strconv.Atoi(pins["CLAUDE_MAX_TURNS"])
	}
	if c.timeout == 0 {
		minutes, _ := strconv.Atoi(pins["WALL_CLOCK_MINUTES"])
		c.timeout = time.Duration(minutes) * time.Minute
	}
	if c.seed == 0 {
		c.seed, _ = strconv.ParseInt(pins["SCHEDULE_SEED"], 10, 64)
	}
	if c.workers == 0 {
		c.workers = 1
		if c.engine == "claude" {
			c.workers = 3
		}
	}
	if c.workspaces == "" {
		c.workspaces = filepath.Join(c.work, "ws")
	}
	if c.out == "" {
		name := c.runName()
		if c.keysRef != "" {
			name += "-keys-" + strings.ReplaceAll(c.keysRef, "/", "-")
		}
		c.out = filepath.Join("bench", "upkeep", "results-"+name+".csv")
	}
	return c
}

// preflightTasks fails early when a scheduled task lacks what a run needs,
// rather than after an agent has already spent its half hour.
func (h *harness) preflightTasks(jobs []job) error {
	seen := map[string]bool{}
	var missing []string
	for _, j := range jobs {
		if seen[j.Task] {
			continue
		}
		seen[j.Task] = true
		dir := filepath.Join(h.tasksDir, j.Task)
		needs := []string{"prompt.md", "key.json"}
		if calibration(h.engine) {
			needs = []string{"key.json"}
		}
		for _, name := range needs {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				missing = append(missing, filepath.Join(dir, name))
			}
		}
		if hidden, _ := filepath.Glob(filepath.Join(dir, "hidden", "*.dart")); len(hidden) == 0 {
			missing = append(missing, filepath.Join(dir, "hidden", "*.dart"))
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

// writeMeta records everything that identifies this invocation next to its
// runs, so a result can be traced to the seed, pin and tool versions.
func (h *harness) writeMeta(jobs []job) error {
	meta := map[string]any{
		"engine": h.engine, "seed": h.seed, "stage": h.stage, "per_stage": h.perStage,
		"fixture_ref": h.ref, "fixture_sha": h.pinSHA, "cinch": h.cinchBin,
		"cinch_version": h.pins["CINCH_VERSION"], "timeout": h.timeout.String(),
		"pi":     map[string]string{"provider": h.provider, "model": h.model, "thinking": h.thinking, "version": toolVersion("pi")},
		"claude": map[string]any{"model": h.claudeModel, "max_turns": h.maxTurns, "version": toolVersion("claude")},
		"runs":   len(jobs), "started": time.Now().Format(time.RFC3339),
	}
	name := fmt.Sprintf("meta-stage%d.json", h.stage)
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
	logf("%s: %s behavior=%s corpus=%s upkeep=%s committed=%t bypass=%t %s", j.id(), res.Status,
		res.Behavior, res.Corpus, res.Upkeep, res.Committed, res.Bypass, strings.Join(res.Invalid, "; "))
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

// writeCSV rebuilds the table from every result.json under the engine's runs,
// invalid first attempts included, so nothing is silently dropped.
func (h *harness) writeCSV() error {
	h.csvMutex.Lock()
	defer h.csvMutex.Unlock()
	paths, err := filepath.Glob(filepath.Join(h.runsDir, "*", "result.json"))
	if err != nil {
		return err
	}
	var all []result
	for _, path := range paths {
		res, err := readResult(filepath.Dir(path))
		if err != nil {
			return err
		}
		all = append(all, res)
	}
	return h.writeTable(all)
}

// writeTable writes results to the CSV at -o, in shuffle order.
func (h *harness) writeTable(all []result) error {
	sort.Slice(all, func(a, b int) bool {
		if all[a].Position != all[b].Position {
			return all[a].Position < all[b].Position
		}
		return all[a].Attempt < all[b].Attempt
	})
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "# engine %s, seed %d; columns are documented in bench/upkeep/main.go\n", h.engine, h.seed)
	writer := csv.NewWriter(&buffer)
	writer.Write([]string{"run_id", "engine", "task", "arm", "index", "attempt", "position", "seed", "base_sha", "head_sha",
		"status", "invalid_reason", "timed_out", "exit_code", "wall_s", "engine_error",
		"turns", "input_tokens", "output_tokens", "cache_read", "cost_usd",
		"committed", "commits", "dirty", "behavior", "hidden_passed", "hidden_failed",
		"corpus", "corpus_reasons", "upkeep", "bypass", "bypass_kinds", "hook_runs", "hook_blocks",
		"rules_changed", "rules_changed_ids", "suite", "suite_failed", "notes", "corpus_first", "upkeep_first"})
	for _, r := range all {
		writer.Write([]string{r.RunID, r.Engine, r.Task, fmt.Sprint(r.Arm), fmt.Sprint(r.Index), fmt.Sprint(r.Attempt),
			fmt.Sprint(r.Position), fmt.Sprint(r.Seed), r.BaseSHA, r.HeadSHA,
			r.Status, strings.Join(r.Invalid, "; "), fmt.Sprint(r.TimedOut), fmt.Sprint(r.ExitCode), fmt.Sprint(r.WallMS / 1000), r.EngineError,
			fmt.Sprint(r.Turns), fmt.Sprint(r.Input), fmt.Sprint(r.Output), fmt.Sprint(r.CacheRead), fmt.Sprintf("%.4f", r.CostUSD),
			fmt.Sprint(r.Committed), fmt.Sprint(r.Commits), fmt.Sprint(r.Dirty), r.Behavior, fmt.Sprint(r.HiddenPassed), fmt.Sprint(r.HiddenFailed),
			r.Corpus, strings.Join(r.CorpusReasons, "; "), r.Upkeep, fmt.Sprint(r.Bypass), strings.Join(r.BypassKinds, " "),
			fmt.Sprint(r.HookRuns), fmt.Sprint(r.HookBlocks),
			fmt.Sprint(len(r.RulesChanged)), strings.Join(r.RulesChanged, " "), r.Suite, fmt.Sprint(r.SuiteFailed), strings.Join(r.Notes, "; "), r.CorpusFirst, r.UpkeepFirst})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(h.out, buffer.Bytes(), 0o644)
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
