// Command recover runs the recoverability census (plan 29, item B): for each
// rule of a repo, a question whose correct answer is the rule's content is put
// to an agent that has only the code, and to one that also has the docs. A
// judge from another model family grades the answers against the rule text.
// A rule the docs arm gets right and the code arm does not is normative: the
// docs carry something the code does not.
//
// Usage:
//
//	recover [run] -questions FILE -arm code,docs -engine pi|fake -reps N
//	        [-repo PATH] [-ref REF] [-gen CMD] [-objectbox DIR] [-flutter DIR]
//	        [-work DIR] [-o results.csv] [-seed N] [-timeout 10m] [-ids A,B]
//	        [-include-ignored] [-dry-run] [-fake-script FILE]
//	recover judge   -results results.csv -rubric rubric.md -judge claude|fake [-model M]
//	recover summary -results results.csv [-reps 3] [-min-correct 2]
//	recover calibrate [-judge claude|fake] [-questions A.json,B.json] [-probes FILE] [-rubric rubric.md]
//
// Every agent process runs in a bubblewrap sandbox, identical in both arms:
// the checkout at /work (a fresh one-commit repository, so history cannot give
// the docs back), an empty home holding only what pi needs, the toolchain,
// system dirs read-only, the network shared, and no cinch anywhere.
//
// Layout under -work: base-<repo>-<sha>/ is the cached, generated checkout
// every run is copied from; runs/<repo>/<engine>/<run>/ holds each run's
// result.json, answer.txt, transcript and logs; agents' checkouts live under
// -ws at an opaque path.
//
// CSV columns (one row per question, arm and rep, ordered by schedule position;
// bench/recover/README.md has the full list): run_id, repo, commit, engine,
// question_id, doc, ignored, arm, rep, position, seed, predict; status (ok, timeout,
// error or empty), timed_out, exit_code, wall_s, engine_error, turns,
// input_tokens, output_tokens, cache_read, cache_write, tool_calls, cost_usd;
// question, key, notes, answer, judged_answer; verdict, justification,
// judge_error, judge_model (written by judge).
package main

import (
	"bytes"
	"context"
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

type config struct {
	repo, ref, pinsPath, work, workspaces, out, engine string
	questionsPath, gen, objectbox, flutter             string
	provider, model, thinking                          string
	reps, workers                                      int
	seed                                               int64
	timeout                                            time.Duration
	armFilter, idFilter                                []string
	piNode, piCLI, fakeScript                          string
	dryRun, keepWork, includeIgnored                   bool
}

// job is one scheduled run. Position is its place in the full shuffle of every
// question, arm and rep, so a run keeps its slot whichever subset is selected.
type job struct {
	Position int
	Q        question
	Arm      string
	Rep      int
}

func (j job) id() string { return fmt.Sprintf("%s-%s-r%d", j.Q.ID, j.Arm, j.Rep) }

func main() {
	args := os.Args[1:]
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "run":
		err = runCommand(args)
	case "judge":
		err = judgeCommand(args)
	case "summary":
		err = summaryCommand(args)
	case "calibrate":
		err = calibrateCommand(args)
	default:
		err = fmt.Errorf("unknown command %q (want run, judge, summary or calibrate)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "recover:", err)
		os.Exit(1)
	}
}

func runCommand(args []string) error {
	home, _ := os.UserHomeDir()
	var c config
	var arms, ids string
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&c.questionsPath, "questions", "", "questions file (bench/recover/questions/<repo>.json)")
	fs.StringVar(&c.pinsPath, "pins", filepath.Join("bench", "recover", "PINS"), "pin of record: fixture SHAs, engine settings, seed")
	fs.StringVar(&c.repo, "repo", "", "fixture repo, bare or working, fetched from and never written to")
	fs.StringVar(&c.ref, "ref", "", "fixture ref (default <REPO>_REF from PINS); must resolve to <REPO>_SHA")
	fs.StringVar(&arms, "arm", "code,docs", "arms to run: code, docs or both")
	fs.StringVar(&c.engine, "engine", "", "pi, or fake (a script stands in for the model)")
	fs.IntVar(&c.reps, "reps", 0, "runs per question and arm (default REPS from PINS)")
	fs.StringVar(&c.work, "work", defaultWork(), "cache directory for the base checkout and per-run outputs")
	fs.StringVar(&c.workspaces, "ws", "", "where agents' checkouts live (default <work>/ws)")
	fs.StringVar(&c.out, "o", "", "CSV output (default bench/recover/results-<repo>-<engine>.csv)")
	fs.Int64Var(&c.seed, "seed", 0, "schedule seed (default SCHEDULE_SEED from PINS)")
	fs.StringVar(&c.gen, "gen", "", "generation command run once in the base checkout, e.g. \"make gen\" (default <REPO>_GEN from PINS)")
	fs.StringVar(&c.objectbox, "objectbox", "", "directory with libobjectbox.so and VERSION, copied into .objectbox/lib (default <REPO>_OBJECTBOX from PINS)")
	fs.StringVar(&c.flutter, "flutter", filepath.Join(home, "dev", "flutter"), "Flutter SDK root, on PATH when it exists (\"\" for none)")
	fs.DurationVar(&c.timeout, "timeout", 0, "wall clock per agent run (default WALL_CLOCK_MINUTES from PINS)")
	fs.IntVar(&c.workers, "workers", 1, "parallel runs (pi has one slot; keep 1)")
	fs.StringVar(&ids, "ids", "", "comma-separated question IDs to run (default all)")
	fs.StringVar(&c.provider, "provider", "", "pi provider (default PI_PROVIDER)")
	fs.StringVar(&c.model, "model", "", "pi model (default PI_MODEL)")
	fs.StringVar(&c.thinking, "thinking", "", "pi thinking level (default PI_THINKING)")
	fs.StringVar(&c.piNode, "pi-node", "", "node binary that runs pi (default: mise's node@latest)")
	fs.StringVar(&c.piCLI, "pi-cli", "", "pi's cli.js (default: the newest under ~/.npm/_npx)")
	fs.StringVar(&c.fakeScript, "fake-script", "", "with -engine fake: a bash script run in the sandbox in place of a model; its stdout is the answer")
	fs.BoolVar(&c.includeIgnored, "include-ignored", false, "also ask the questions marked \"ignored\" (DESIGN's secondary row); changes the schedule, so decide once per repo")
	fs.BoolVar(&c.dryRun, "dry-run", false, "print the schedule and exit")
	fs.BoolVar(&c.keepWork, "keep-work", false, "keep each agent checkout after the run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	c.armFilter, c.idFilter = splitList(arms), splitList(ids)
	for _, arm := range c.armFilter {
		if !contains(canonicalArms, arm) {
			return fmt.Errorf("-arm wants code or docs, got %q", arm)
		}
	}
	return run(c, set)
}

func defaultWork() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cinch-recover")
	}
	return filepath.Join(os.TempDir(), "cinch-recover")
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

func contains(list []string, item string) bool {
	for _, existing := range list {
		if existing == item {
			return true
		}
	}
	return false
}

// schedule is the full randomized order of every question, arm and rep for one
// engine. The shuffle is a Fisher-Yates over PCG output with the modulo taken
// by hand: the standard library documents neither Shuffle's algorithm nor
// IntN's as stable, and this order is pre-registered.
func schedule(seed int64, engine string, qs []question, reps int) []job {
	var jobs []job
	for _, q := range qs {
		for _, arm := range canonicalArms {
			for rep := 0; rep < reps; rep++ {
				jobs = append(jobs, job{Q: q, Arm: arm, Rep: rep})
			}
		}
	}
	shuffle(seed, engine, len(jobs), func(i, j int) { jobs[i], jobs[j] = jobs[j], jobs[i] })
	for i := range jobs {
		jobs[i].Position = i
	}
	return jobs
}

// shuffle is a Fisher-Yates over PCG output seeded by seed and a name.
func shuffle(seed int64, name string, n int, swap func(i, j int)) {
	hasher := fnv.New64a()
	hasher.Write([]byte(name))
	source := rand.NewPCG(uint64(seed), hasher.Sum64())
	for i := n - 1; i > 0; i-- {
		swap(i, int(source.Uint64()%uint64(i+1)))
	}
}

// selectJobs keeps the requested arms and question IDs, in shuffle order.
func selectJobs(all []job, arms, ids []string) []job {
	var out []job
	for _, j := range all {
		if arms != nil && !contains(arms, j.Arm) {
			continue
		}
		if ids != nil && !contains(ids, j.Q.ID) {
			continue
		}
		out = append(out, j)
	}
	return out
}

// harness is the state every run shares.
type harness struct {
	config
	pins      map[string]string
	arms      armSpec
	questions *questionFile
	pinSHA    string
	baseDir   string
	sandbox   sandboxPaths
	baseEnv   []string
	runsDir   string
	csvMutex  sync.Mutex
}

func run(c config, set map[string]bool) error {
	pins, err := loadPins(c.pinsPath)
	if err != nil {
		return err
	}
	if c.questionsPath == "" {
		return fmt.Errorf("-questions is required")
	}
	qf, err := loadQuestions(c.questionsPath)
	if err != nil {
		return err
	}
	if c.engine != "pi" && c.engine != "fake" {
		return fmt.Errorf("-engine wants pi or fake")
	}
	c = withPinDefaults(c, pins, qf.Repo, set)
	qs, err := qf.selected(c.includeIgnored)
	if err != nil {
		return err
	}
	for _, id := range c.idFilter {
		if !contains(questionIDs(qs), id) {
			return fmt.Errorf("-ids: %q is not a scheduled question of %s (ignored ones need -include-ignored)", id, c.questionsPath)
		}
	}
	h := &harness{config: c, pins: pins, questions: qf}
	all := schedule(c.seed, c.engine, qs, c.reps)
	jobs := selectJobs(all, c.armFilter, c.idFilter)

	if c.dryRun {
		fmt.Printf("# %s, engine %s, seed %d, reps %d, %d runs\n", qf.Repo, c.engine, c.seed, c.reps, len(jobs))
		for i, j := range jobs {
			fmt.Printf("%3d  pos %3d  %-16s question %s  arm %s  rep %d\n", i+1, j.Position, j.id(), j.Q.ID, j.Arm, j.Rep)
		}
		return nil
	}

	if c.repo == "" {
		return fmt.Errorf("-repo is required: the fixture repo to fetch %s from", qf.Repo)
	}
	ref, sha, err := fixturePin(pins, qf.Repo)
	if err != nil {
		return err
	}
	if h.ref == "" {
		h.ref = ref
	}
	h.pinSHA = sha
	if h.arms, err = armSpecFor(pins, filepath.Dir(c.pinsPath), qf.Repo); err != nil {
		return err
	}
	if err := h.setup(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	h.runsDir = filepath.Join(c.work, "runs", qf.Repo, c.engine)
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

func questionIDs(qs []question) []string {
	var out []string
	for _, q := range qs {
		out = append(out, q.ID)
	}
	return out
}

func withPinDefaults(c config, pins map[string]string, repo string, set map[string]bool) config {
	prefix := pinPrefix(repo)
	pickString := func(current *string, key string) {
		if *current == "" {
			*current = pins[key]
		}
	}
	pickString(&c.ref, prefix+"_REF")
	if c.ref == "TODO" {
		c.ref = ""
	}
	if !set["gen"] {
		c.gen = pins[prefix+"_GEN"]
	}
	if !set["objectbox"] {
		c.objectbox = expandHome(pins[prefix+"_OBJECTBOX"])
	}
	pickString(&c.provider, "PI_PROVIDER")
	pickString(&c.model, "PI_MODEL")
	pickString(&c.thinking, "PI_THINKING")
	if c.reps == 0 {
		c.reps, _ = strconv.Atoi(pins["REPS"])
	}
	if c.timeout == 0 {
		minutes, _ := strconv.Atoi(pins["WALL_CLOCK_MINUTES"])
		c.timeout = time.Duration(minutes) * time.Minute
	}
	if c.seed == 0 {
		c.seed, _ = strconv.ParseInt(pins["SCHEDULE_SEED"], 10, 64)
	}
	if c.workers < 1 {
		c.workers = 1
	}
	if c.workspaces == "" {
		c.workspaces = filepath.Join(c.work, "ws")
	}
	if c.out == "" {
		c.out = filepath.Join("bench", "recover", fmt.Sprintf("results-%s-%s.csv", repo, c.engine))
	}
	return c
}

func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}

// writeMeta records everything that identifies this invocation next to its
// runs, so a result can be traced to the seed, pin and tool versions.
func (h *harness) writeMeta(jobs []job) error {
	meta := map[string]any{
		"repo": h.questions.Repo, "commit": h.questions.Commit, "engine": h.engine, "seed": h.seed, "reps": h.reps,
		"arms": h.armFilter, "include_ignored": h.includeIgnored, "docs_root": h.arms.docsRoot, "fixture_ref": h.ref, "fixture_sha": h.pinSHA, "gen": h.gen, "timeout": h.timeout.String(),
		"pi":   map[string]string{"provider": h.provider, "model": h.model, "thinking": h.thinking, "version": toolVersion("pi")},
		"runs": len(jobs), "started": time.Now().Format(time.RFC3339),
	}
	return writeJSON(filepath.Join(h.runsDir, fmt.Sprintf("meta-%d.json", time.Now().Unix())), meta)
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
// no result.json, so it is cleared and redone.
func (h *harness) runOne(ctx context.Context, j job) error {
	dir := filepath.Join(h.runsDir, j.id())
	if _, err := readResult(dir); err == nil {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	res, err := h.execute(ctx, j, dir)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "result.json"), res); err != nil {
		return err
	}
	logf("%s: %s turns=%d wall=%ds answer=%d bytes", j.id(), res.Status, res.Turns, res.WallMS/1000, len(res.Answer))
	return h.writeCSV()
}

// execute provisions the arm, runs the agent and reads its answer.
func (h *harness) execute(ctx context.Context, j job, dir string) (result, error) {
	repo, err := h.provision(j, 1)
	if err != nil {
		return result{}, err
	}
	if !h.keepWork {
		defer os.RemoveAll(filepath.Dir(repo))
	}
	ended, err := h.runAgent(ctx, repo, dir, buildPrompt(j.Q.Question))
	if err != nil {
		return result{}, err
	}
	tr, err := parseTranscript(h.engine, filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		return result{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "answer.txt"), []byte(tr.FinalText+"\n"), 0o644); err != nil {
		return result{}, err
	}
	res := result{
		RunID: j.id(), Repo: h.questions.Repo, Commit: h.questions.Commit, Engine: h.engine,
		QuestionID: j.Q.ID, Doc: j.Q.Doc, Ignored: j.Q.Ignored, Arm: j.Arm, Rep: j.Rep, Position: j.Position, Seed: h.seed, Predict: j.Q.Predict,
		TimedOut: ended.TimedOut, ExitCode: ended.ExitCode, WallMS: ended.WallMS, EngineError: tr.EngineError,
		Turns: tr.Turns, Input: tr.Input, Output: tr.Output, CacheRead: tr.CacheRead, CacheWrite: tr.CacheWrite,
		ToolCalls: tr.ToolCalls, CostUSD: tr.CostUSD,
		Question: j.Q.Question, Key: j.Q.Key, Notes: j.Q.Notes, Answer: tr.FinalText,
	}
	switch {
	case ended.TimedOut:
		res.Status = "timeout"
	case res.EngineError != "" || (ended.ExitCode != 0 && tr.FinalText == ""):
		res.Status = "error"
	case tr.FinalText == "":
		res.Status = "empty"
	default:
		res.Status = "ok"
	}
	return res, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// sortResults orders rows by schedule position, then by name.
func sortResults(all []result) {
	sort.Slice(all, func(a, b int) bool {
		if all[a].Position != all[b].Position {
			return all[a].Position < all[b].Position
		}
		return all[a].RunID < all[b].RunID
	})
}
