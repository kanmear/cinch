package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The rule-written judge (DESIGN § The judge): a model sees one invariant (the
// task's, from its NOTES) and one candidate rule text, and says whether the rule
// STATES it, PARTIALLY states it, or is ABSENT from it. It never sees the arm,
// the transcript, the code, the task's other rules or the hidden-test result.
// The code is bench/recover's judge path (rubric fill, verdict parse, one
// retry), adapted to this vocabulary.

var verdicts = []string{"STATES", "PARTIAL", "ABSENT"}

// rubricFields are the placeholders a rubric must contain, and the only things
// a judge is shown.
var rubricFields = []string{"invariant", "rule"}

// judgeInput is everything a judge sees.
type judgeInput struct {
	Invariant string
	Rule      string
}

// fillRubric substitutes {{invariant}} and {{rule}}. A rubric without both is
// refused: a judge that never sees the invariant, or the rule, grades nothing.
func fillRubric(rubric string, in judgeInput) (string, error) {
	values := map[string]string{"invariant": in.Invariant, "rule": in.Rule}
	for _, field := range rubricFields {
		if !strings.Contains(rubric, "{{"+field+"}}") {
			return "", fmt.Errorf("the rubric has no {{%s}} placeholder", field)
		}
	}
	// One pass, so a value that itself contains "{{rule}}" is not expanded.
	pairs := make([]string, 0, 2*len(values))
	for field, value := range values {
		pairs = append(pairs, "{{"+field+"}}", value)
	}
	return strings.NewReplacer(pairs...).Replace(rubric), nil
}

var verdictLineRe = regexp.MustCompile(`^[\s*#>_` + "`" + `-]*(?:(?i:(?:final\s+)?verdict)\s*[:\-–—]?\s*)?[*_` + "`" + `]*(STATES|PARTIAL|ABSENT)\b[*_` + "`" + `]*\s*[:.\-–—]?\s*(.*)$`)
var verdictWordRe = regexp.MustCompile(`\b(STATES|PARTIAL|ABSENT)\b`)

// parseVerdict reads a judge's reply: exactly one of STATES, PARTIAL or ABSENT
// (upper case), and the justification. The verdict is the first line that opens
// with one (optionally after "Verdict:" and markdown); the justification is the
// rest of that line and what follows. A reply that names a second, different
// verdict on a line of its own, or none at all, is an error.
func parseVerdict(reply string) (verdict, justification string, err error) {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	for i, line := range lines {
		m := verdictLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		verdict = m[1]
		rest := append([]string{strings.TrimSpace(m[2])}, lines[i+1:]...)
		justification = strings.TrimSpace(strings.Join(rest, "\n"))
		for _, later := range lines[i+1:] {
			if l := verdictLineRe.FindStringSubmatch(later); l != nil && l[1] != verdict {
				return "", "", fmt.Errorf("ambiguous verdict: %s, then %s", verdict, l[1])
			}
		}
		return verdict, justification, nil
	}
	seen := map[string]bool{}
	for _, w := range verdictWordRe.FindAllString(reply, -1) {
		seen[w] = true
	}
	if len(seen) == 1 {
		for w := range seen {
			return w, strings.TrimSpace(reply), nil
		}
	}
	return "", "", fmt.Errorf("no single verdict in the reply: %q", truncate(reply, 200))
}

var (
	ruleNumRe    = regexp.MustCompile(`^\s*\d+\.\s+`)
	ruleIDRe     = regexp.MustCompile(`^\*\*[A-Za-z0-9-]+\*\*\s*`)
	ruleIgnoreRe = regexp.MustCompile(`(?s)\s*<!--\s*cinch:ignore.*?-->`)
)

// ruleText is a rule as the judge sees it: the list number, the bold ID and any
// cinch:ignore comment stripped.
func ruleText(text string) string {
	s := ruleIgnoreRe.ReplaceAllString(text, "")
	s = ruleNumRe.ReplaceAllString(s, "")
	s = ruleIDRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// judger grades one candidate. It sees the filled rubric's inputs only.
type judger interface {
	judge(ctx context.Context, rubric string, in judgeInput) (verdict, justification string, err error)
	name() string
}

// fakeJudge is the deterministic stub the tests and dry runs use: STATES iff the
// rule text contains the invariant's first 12 characters, else ABSENT.
type fakeJudge struct{}

func (fakeJudge) name() string { return "fake" }
func (fakeJudge) judge(_ context.Context, _ string, in judgeInput) (string, string, error) {
	prefix := []rune(in.Invariant)
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	if strings.Contains(in.Rule, string(prefix)) {
		return "STATES", "fake judge: the rule contains the invariant's first 12 characters", nil
	}
	return "ABSENT", "fake judge: the rule lacks the invariant's first 12 characters", nil
}

// claudeJudge calls claude -p non-interactively, with no tools, in an empty
// directory with only project-level settings, so no CLAUDE.md or memory of the
// user's reaches the grader.
type claudeJudge struct {
	model   string
	bin     string
	timeout time.Duration
}

func (j claudeJudge) name() string { return j.model }

func (j claudeJudge) judge(ctx context.Context, rubric string, in judgeInput) (string, string, error) {
	prompt, err := fillRubric(rubric, in)
	if err != nil {
		return "", "", err
	}
	scratch, err := os.MkdirTemp("", "leak-judge-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(scratch)
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, j.bin, "-p", "--output-format", "json", "--model", j.model,
		"--tools", "", "--no-session-persistence", "--disable-slash-commands", "--strict-mcp-config",
		"--setting-sources", "project")
	cmd.Dir = scratch
	cmd.Env = append(cleanEnv(os.Environ()), "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1", "DISABLE_AUTOUPDATER=1")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("claude: %v: %s", err, truncate(strings.TrimSpace(stderr.String()), 300))
	}
	reply, err := replyText(stdout.Bytes())
	if err != nil {
		return "", "", err
	}
	return parseVerdict(reply)
}

// replyText extracts the model's text from claude's --output-format json: one
// result object (or an array of events whose last is the result).
func replyText(data []byte) (string, error) {
	type resultLine struct {
		Type    string `json:"type"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	var one resultLine
	if err := json.Unmarshal(data, &one); err != nil {
		var many []resultLine
		if json.Unmarshal(data, &many) != nil || len(many) == 0 {
			return "", fmt.Errorf("claude printed no JSON result: %q", truncate(string(data), 200))
		}
		one = many[len(many)-1]
	}
	if one.IsError {
		return "", fmt.Errorf("claude reported an error: %s", truncate(one.Result, 300))
	}
	if strings.TrimSpace(one.Result) == "" {
		return "", fmt.Errorf("claude returned an empty result")
	}
	return one.Result, nil
}

// piJudge asks the pi/Qwen model, with no tools, in an empty directory. It
// judges the Claude runs (a different family from the agent).
type piJudge struct {
	node, cli, provider, model, thinking string
	timeout                              time.Duration
}

func (j piJudge) name() string { return "pi:" + j.model }

func (j piJudge) judge(ctx context.Context, rubric string, in judgeInput) (string, string, error) {
	prompt, err := fillRubric(rubric, in)
	if err != nil {
		return "", "", err
	}
	scratch, err := os.MkdirTemp("", "leak-judge-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(scratch)
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, j.node, j.cli, "-p", "--mode", "json",
		"--provider", j.provider, "--model", j.model, "--thinking", j.thinking,
		"--no-session", "--offline", "--no-tools", "-ne", "-ns", "-np", "-nc", "--", prompt)
	cmd.Dir = scratch
	cmd.Env = cleanEnv(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("pi: %v: %s", err, truncate(strings.TrimSpace(stderr.String()), 300))
	}
	dir, err := os.MkdirTemp("", "leak-judge-out-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "t.jsonl")
	if err := os.WriteFile(path, stdout.Bytes(), 0o644); err != nil {
		return "", "", err
	}
	tr, err := parseTranscript("pi", path)
	if err != nil {
		return "", "", err
	}
	if tr.EngineError != "" || strings.TrimSpace(tr.FinalText) == "" {
		return "", "", fmt.Errorf("pi judge: no reply (%s)", tr.EngineError)
	}
	return parseVerdict(tr.FinalText)
}

// judgeOne grades one candidate the way every judge path does: an empty rule
// is ABSENT without a call, and a failed or unparseable reply is retried once.
func judgeOne(ctx context.Context, rubric string, j judger, in judgeInput) (verdict, why string, err error) {
	if strings.TrimSpace(in.Rule) == "" {
		return "ABSENT", "no rule text", nil
	}
	if verdict, why, err = j.judge(ctx, rubric, in); err != nil {
		verdict, why, err = j.judge(ctx, rubric, in)
	}
	return verdict, why, err
}

var invariantRe = regexp.MustCompile(`Invariant:\s*"([^"]+)"`)

// invariantOf reads a task's invariant from its NOTES.md ("Invariant: "..."").
func invariantOf(taskDirPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(taskDirPath, "NOTES.md"))
	if err != nil {
		return "", err
	}
	m := invariantRe.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("%s/NOTES.md has no Invariant: \"...\"", taskDirPath)
	}
	return strings.Join(strings.Fields(string(m[1])), " "), nil
}

// makeJudge builds the judge a command asks for. "auto" is the other family
// from the engine whose runs are judged: Claude judges pi runs, pi judges Claude's.
func makeJudge(kind, forEngine, model string, pins map[string]string, piNode, piCLI string) (judger, error) {
	if kind == "auto" {
		kind = "claude"
		if forEngine == "claude" {
			kind = "pi"
		}
	}
	switch kind {
	case "fake":
		return fakeJudge{}, nil
	case "claude":
		bin, err := exec.LookPath("claude")
		if err != nil {
			return nil, fmt.Errorf("claude: %w", err)
		}
		if model == "" {
			model = pins["JUDGE_FOR_PI"]
		}
		if model == "" {
			model = "claude-sonnet-5-5"
		}
		return claudeJudge{model: model, bin: bin, timeout: 5 * time.Minute}, nil
	case "pi":
		home, _ := os.UserHomeDir()
		node, cli, err := findPi(home, piNode, piCLI)
		if err != nil {
			return nil, err
		}
		if model == "" {
			model = pins["PI_MODEL"]
		}
		return piJudge{node: node, cli: cli, provider: pins["PI_PROVIDER"], model: model, thinking: pins["PI_THINKING"], timeout: 10 * time.Minute}, nil
	}
	return nil, fmt.Errorf("-judge wants claude, pi, fake or auto, got %q", kind)
}

// candidateRef is one candidate rule of one run, awaiting a verdict.
type candidateRef struct {
	RunDir string
	RunID  string
	Task   string
	Cand   ruleCandidate
}

// collectCandidates lists the candidates of every run under runsDir that has no
// verdict yet (or all, with redo), in shuffled order, so no arm is judged in a
// block. T4 only has the add-path candidates judged.
func collectCandidates(runsDir string, seed int64, redo bool) ([]candidateRef, error) {
	paths, err := filepath.Glob(filepath.Join(runsDir, "*", "result.json"))
	if err != nil {
		return nil, err
	}
	var out []candidateRef
	for _, path := range paths {
		dir := filepath.Dir(path)
		res, err := readResult(dir)
		if err != nil {
			return nil, err
		}
		have := readJudge(dir)
		for _, c := range res.RuleCandidates {
			if res.Task == "T4" && !c.AddPath {
				continue
			}
			if _, ok := have[c.ID]; ok && !redo {
				continue
			}
			out = append(out, candidateRef{RunDir: dir, RunID: res.RunID, Task: res.Task, Cand: c})
		}
	}
	// A stable base order, then the seeded shuffle.
	sortCandidates(out)
	return permute(out, seed), nil
}

// permute applies the seeded shuffle to a slice of candidates. (It is the same
// Fisher-Yates as the schedule, over indices.)
func permute(in []candidateRef, seed int64) []candidateRef {
	idx := make([]job, len(in))
	for i := range idx {
		idx[i] = job{Position: i, Index: i}
	}
	// Fisher-Yates over idx with the judge stream: shuffleJobs reorders and then
	// renumbers Position, so Index still names the source element.
	shuffled := shuffleJobs(idx, seed, "judge", "")
	out := make([]candidateRef, len(in))
	for i, j := range shuffled {
		out[i] = in[j.Index]
	}
	return out
}

func sortCandidates(c []candidateRef) {
	for i := 1; i < len(c); i++ {
		for k := i; k > 0 && (c[k-1].RunID+"/"+c[k-1].Cand.ID > c[k].RunID+"/"+c[k].Cand.ID); k-- {
			c[k-1], c[k] = c[k], c[k-1]
		}
	}
}

func judgeMain(args []string) error {
	var c config
	var kind, model, rubricPath, forEngine string
	var workers, repeatPct, audit int
	var redo bool
	fs := flag.NewFlagSet("judge", flag.ExitOnError)
	commonFlags(fs, &c)
	fs.StringVar(&forEngine, "engine", "", "the engine whose runs are judged (names the runs directory; selects -judge auto)")
	fs.BoolVar(&c.pilot, "pilot", false, "judge the pilot runs")
	fs.StringVar(&kind, "judge", "auto", "claude (the pi runs' judge), pi (the claude runs' judge), fake, or auto")
	fs.StringVar(&model, "model", "", "judge model (default JUDGE_FOR_PI for claude, PI_MODEL for pi)")
	fs.StringVar(&rubricPath, "rubric", filepath.Join("bench", "leak", "rubric.md"), "rubric file with {{invariant}} and {{rule}}")
	fs.IntVar(&workers, "workers", 3, "parallel judge calls")
	fs.IntVar(&repeatPct, "repeat-pct", 10, "judge this seeded share of the candidates a second time and report agreement (DESIGN § The judge)")
	fs.IntVar(&audit, "audit", 0, "write a blind audit sheet of N seeded candidates (rule text only) for the owner, and exit")
	fs.BoolVar(&redo, "redo", false, "judge every candidate again, keeping nothing")
	fs.StringVar(&c.piNode, "pi-node", "", "node binary that runs pi")
	fs.StringVar(&c.piCLI, "pi-cli", "", "pi's cli.js")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if forEngine == "" {
		return fmt.Errorf("-engine is required: the engine whose runs are judged")
	}
	c.engine = forEngine
	h, err := newHarness(c)
	if err != nil {
		return err
	}
	h.runsDir = filepath.Join(h.work, "runs", h.runName())
	cands, err := collectCandidates(h.runsDir, h.seed, redo)
	if err != nil {
		return err
	}
	if audit > 0 {
		return writeAudit(h, cands, audit)
	}
	j, err := makeJudge(kind, forEngine, model, h.pins, c.piNode, c.piCLI)
	if err != nil {
		return err
	}
	rubric := ""
	if kind != "fake" {
		data, err := os.ReadFile(rubricPath)
		if err != nil {
			return fmt.Errorf("rubric: %w", err)
		}
		rubric = string(data)
	}
	if err := judgeAll(context.Background(), h, cands, rubric, j, workers, repeatPct); err != nil {
		return err
	}
	return h.writeCSV()
}

// judgeAll grades every candidate without a verdict, saving each run's
// judge.json after each call so an interrupted judge resumes where it stopped.
func judgeAll(ctx context.Context, h *harness, cands []candidateRef, rubric string, j judger, workers, repeatPct int) error {
	logf("%d candidate rules to judge with %s", len(cands), j.name())
	var mu sync.Mutex
	invariants := map[string]string{}
	for _, task := range canonicalTasks {
		if dir, _, err := taskDir(h.leakDir, task); err == nil {
			if inv, err := invariantOf(dir); err == nil {
				invariants[task] = inv
			}
		}
	}
	saved := map[string]map[string]judged{}
	load := func(dir string) map[string]judged {
		if m, ok := saved[dir]; ok {
			return m
		}
		m := readJudge(dir)
		saved[dir] = m
		return m
	}
	repeat := map[string]judged{} // run id + candidate -> second verdict
	if workers < 1 {
		workers = 1
	}
	// The seeded sample judged twice.
	again := map[int]bool{}
	if repeatPct > 0 {
		for i := range cands {
			if sampled(h.seed, cands[i].RunID+"/"+cands[i].Cand.ID, repeatPct) {
				again[i] = true
			}
		}
	}
	queue := make(chan int)
	var wg sync.WaitGroup
	failed := 0
	var saveErr error
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				ref := cands[i]
				inv := invariants[ref.Task]
				in := judgeInput{Invariant: inv, Rule: ruleText(ref.Cand.Text)}
				verdict, why, err := judgeOne(ctx, rubric, j, in)
				var second judged
				if err == nil && again[i] {
					if v2, w2, err2 := judgeOne(ctx, rubric, j, in); err2 == nil {
						second = judged{Verdict: v2, Why: w2, Judge: j.name()}
					}
				}
				mu.Lock()
				if err != nil {
					failed++
					logf("%s/%s: judge failed: %v", ref.RunID, ref.Cand.ID, err)
				} else {
					m := load(ref.RunDir)
					m[ref.Cand.ID] = judged{Verdict: verdict, Why: why, Judge: j.name()}
					if second.Verdict != "" {
						repeat[ref.RunID+"/"+ref.Cand.ID] = second
					}
					if werr := writeJSON(filepath.Join(ref.RunDir, judgeFile), m); werr != nil && saveErr == nil {
						saveErr = werr
					}
				}
				mu.Unlock()
			}
		}()
	}
	for i := range cands {
		if ctx.Err() != nil {
			break
		}
		queue <- i
	}
	close(queue)
	wg.Wait()
	if saveErr != nil {
		return saveErr
	}
	if len(repeat) > 0 {
		agree := 0
		for key, second := range repeat {
			for _, ref := range cands {
				if ref.RunID+"/"+ref.Cand.ID == key {
					if load(ref.RunDir)[ref.Cand.ID].Verdict == second.Verdict {
						agree++
					}
				}
			}
		}
		logf("judge repeat: %d of %d re-judged candidates got the same verdict", agree, len(repeat))
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d judge calls failed; run judge again to retry them", failed, len(cands))
	}
	return ctx.Err()
}

// sampled is a seeded, order-independent coin: true for about pct percent of keys.
func sampled(seed int64, key string, pct int) bool {
	return hash64(fmt.Sprint(seed)+"|"+key)%100 < uint64(pct)
}

// writeAudit writes a blind sheet for the owner: N seeded candidates, rule text
// only (no arm, task, engine, run or verdict), to <work>/audit-<runs>.csv, and
// the key beside it.
func writeAudit(h *harness, cands []candidateRef, n int) error {
	if n > len(cands) {
		n = len(cands)
	}
	var sheet, key strings.Builder
	sheet.WriteString("id,invariant,rule_text,owner_verdict\n")
	key.WriteString("id,run_id,candidate\n")
	for i := 0; i < n; i++ {
		ref := cands[i]
		dir, _, _ := taskDir(h.leakDir, ref.Task)
		inv, _ := invariantOf(dir)
		fmt.Fprintf(&sheet, "%d,%s,%s,\n", i+1, csvQuote(inv), csvQuote(ruleText(ref.Cand.Text)))
		fmt.Fprintf(&key, "%d,%s,%s\n", i+1, ref.RunID, ref.Cand.ID)
	}
	base := filepath.Join(h.work, "audit-"+h.runName())
	if err := os.WriteFile(base+".csv", []byte(sheet.String()), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(base+"-key.csv", []byte(key.String()), 0o600); err != nil {
		return err
	}
	logf("wrote %s.csv (blind) and %s-key.csv", base, base)
	return nil
}

// csvQuote quotes a field for a CSV line.
func csvQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// hash64 is FNV-1a over a string.
func hash64(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
