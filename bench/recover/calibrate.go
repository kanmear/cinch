package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Calibration sets, DESIGN § Analysis, Judge checks.
const (
	setOwnText = "own-text" // (a) every rule's own text as the answer
	setAbstain = "abstain"  // (b) the fixed abstain sentence for 10 rules
	setProbe   = "probe"    // (c) the negation and partial probes
)

const (
	ownTextMinPercent = 97 // (a): CORRECT for at least this share of the rules
	abstainRules      = 10 // (b): how many rules get the abstain sentence
)

// probe is one row of DESIGN's probe table: an answer, the rule whose question,
// key and notes it is judged against, and the verdict the judge must give.
type probe struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Repo     string `json:"repo"`
	Answer   string `json:"answer"`
	Expected string `json:"expected"`
}

type probeFile struct {
	Comment string  `json:"comment"`
	Probes  []probe `json:"probes"`
}

func loadProbes(path string) ([]probe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f probeFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("probes: %w", err)
	}
	seen := map[string]bool{}
	for _, p := range f.Probes {
		if p.ID == "" || p.Rule == "" || p.Repo == "" || strings.TrimSpace(p.Answer) == "" || !contains(verdicts, p.Expected) {
			return nil, fmt.Errorf("probes: %q: id, rule, repo, answer and an expected verdict are required", p.ID)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("probes: duplicate id %q", p.ID)
		}
		seen[p.ID] = true
	}
	if len(f.Probes) == 0 {
		return nil, fmt.Errorf("probes: none")
	}
	return f.Probes, nil
}

// calRow is one calibration answer and, once judged, its verdict.
type calRow struct {
	ID, Set, Answer, Expected, Verdict, Justification string
	JudgedAnswer, JudgeError, JudgeModel              string
	input                                             result // the question, key and notes it is judged against
}

func (c calRow) key() string { return c.Set + "|" + c.ID }

var calHeader = []string{"id", "set", "answer", "expected", "verdict", "justification", "judged_answer", "judge_error", "judge_model"}

// buildCalibration makes the three sets from the question files (every rule,
// ignored ones included) and the probes. Set (b)'s rules are the first
// abstainRules of the rules shuffled with the seed.
func buildCalibration(files []*questionFile, probes []probe, seed int64) ([]calRow, error) {
	type rule struct {
		repo string
		q    question
	}
	var rules []rule
	for _, f := range files {
		for _, q := range f.Questions {
			if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.Key) == "" {
				return nil, fmt.Errorf("%s %s: calibration needs a question and a key for every rule", f.Repo, q.ID)
			}
			rules = append(rules, rule{f.Repo, q})
		}
	}
	input := func(r rule, answer string) result {
		return result{Repo: r.repo, QuestionID: r.q.ID, Question: r.q.Question, Key: r.q.Key, Notes: r.q.Notes, Answer: answer}
	}
	var rows []calRow
	for _, r := range rules {
		answer := ruleText(r.q.Key)
		rows = append(rows, calRow{ID: r.repo + ":" + r.q.ID, Set: setOwnText, Answer: answer, Expected: "CORRECT", input: input(r, answer)})
	}
	if len(rules) < abstainRules {
		return nil, fmt.Errorf("only %d rules, set (b) needs %d", len(rules), abstainRules)
	}
	order := append([]rule{}, rules...)
	shuffle(seed, "calibrate-abstain", len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	for _, r := range order[:abstainRules] {
		rows = append(rows, calRow{ID: r.repo + ":" + r.q.ID, Set: setAbstain, Answer: abstainSentence, Expected: "ABSTAIN", input: input(r, abstainSentence)})
	}
	for _, p := range probes {
		var found *rule
		for i := range rules {
			if rules[i].repo == p.Repo && rules[i].q.ID == p.Rule {
				found = &rules[i]
			}
		}
		if found == nil {
			return nil, fmt.Errorf("probe %s: no rule %s in %s", p.ID, p.Rule, p.Repo)
		}
		rows = append(rows, calRow{ID: p.ID, Set: setProbe, Answer: p.Answer, Expected: p.Expected, input: input(*found, p.Answer)})
	}
	return rows, nil
}

func readCalCSV(path string) (map[string]calRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("%s: unreadable", path)
	}
	index := map[string]int{}
	for i, name := range records[0] {
		index[name] = i
	}
	out := map[string]calRow{}
	for _, rec := range records[1:] {
		get := func(name string) string {
			if i, ok := index[name]; ok && i < len(rec) {
				return rec[i]
			}
			return ""
		}
		c := calRow{ID: get("id"), Set: get("set"), Answer: get("answer"), Expected: get("expected"), Verdict: get("verdict"),
			Justification: get("justification"), JudgedAnswer: get("judged_answer"), JudgeError: get("judge_error"), JudgeModel: get("judge_model")}
		out[c.key()] = c
	}
	return out, nil
}

func writeCalCSV(path string, rows []calRow) error {
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)
	w.Write(calHeader)
	for _, c := range rows {
		w.Write([]string{c.ID, c.Set, c.Answer, c.Expected, c.Verdict, c.Justification, c.JudgedAnswer, c.JudgeError, c.JudgeModel})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return writeFileAtomic(path, buffer.Bytes())
}

// calibrate judges every row without a verdict (the file at path is the
// resume state, rewritten after each call), then checks the three sets and
// prints the outcome to w. The error is non-nil when a set fails or any row
// is still unjudged.
func calibrate(ctx context.Context, w io.Writer, path, rubric string, j judger, rows []calRow, workers int, seed int64) error {
	if prior, err := readCalCSV(path); err == nil {
		for i, r := range rows {
			if p, ok := prior[r.key()]; ok && p.Answer == r.Answer && p.Verdict != "" {
				rows[i].Verdict, rows[i].Justification, rows[i].JudgedAnswer, rows[i].JudgeModel = p.Verdict, p.Justification, p.JudgedAnswer, p.JudgeModel
			}
		}
	}
	var todo []int
	for i, r := range rows {
		if r.Verdict == "" {
			todo = append(todo, i)
		}
	}
	shuffle(seed, "judge", len(todo), func(a, b int) { todo[a], todo[b] = todo[b], todo[a] })
	logf("%d of %d calibration answers to judge", len(todo), len(rows))
	var mu sync.Mutex
	var saveErr error
	if workers < 1 {
		workers = 1
	}
	queue := make(chan int)
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				mu.Lock()
				in := rows[i].input
				mu.Unlock()
				verdict, why, judged, err := judgeOne(ctx, rubric, j, in)
				mu.Lock()
				rows[i].JudgedAnswer = judged
				if err != nil {
					rows[i].JudgeError = err.Error()
					logf("%s %s: judge failed: %v", rows[i].Set, rows[i].ID, err)
				} else {
					rows[i].Verdict, rows[i].Justification, rows[i].JudgeError, rows[i].JudgeModel = verdict, why, "", j.name()
				}
				if err := writeCalCSV(path, rows); err != nil && saveErr == nil {
					saveErr = err
				}
				mu.Unlock()
			}
		}()
	}
	for _, i := range todo {
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
	if len(todo) == 0 {
		if err := writeCalCSV(path, rows); err != nil {
			return err
		}
	}
	return reportCalibration(w, rows)
}

// reportCalibration checks the thresholds: (a) CORRECT for at least 97% of the
// rules' own texts, (b) ABSTAIN for all 10 fixed-sentence answers, (c) every
// probe at its expected verdict. A row with no verdict fails its set.
func reportCalibration(w io.Writer, rows []calRow) error {
	type tally struct {
		n, hit int
		fails  []string
	}
	sets := map[string]*tally{setOwnText: {}, setAbstain: {}, setProbe: {}}
	unjudged := 0
	for _, r := range rows {
		t := sets[r.Set]
		t.n++
		switch {
		case r.Verdict == "":
			unjudged++
			t.fails = append(t.fails, r.ID+" (unjudged)")
		case r.Verdict == r.Expected:
			t.hit++
		default:
			t.fails = append(t.fails, fmt.Sprintf("%s (%s, want %s)", r.ID, r.Verdict, r.Expected))
		}
	}
	a, b, c := sets[setOwnText], sets[setAbstain], sets[setProbe]
	results := []struct {
		name, need string
		pass       bool
		t          *tally
	}{
		{"(a) own text judged CORRECT", fmt.Sprintf(">= %d%%", ownTextMinPercent), a.n > 0 && a.hit*100 >= ownTextMinPercent*a.n, a},
		{"(b) fixed abstain sentence judged ABSTAIN", "all", b.n > 0 && b.hit == b.n, b},
		{"(c) probes at their expected verdict", "all", c.n > 0 && c.hit == c.n, c},
	}
	failed := 0
	for _, r := range results {
		status := "PASS"
		if !r.pass {
			status = "FAIL"
			failed++
		}
		pct := 0.0
		if r.t.n > 0 {
			pct = 100 * float64(r.t.hit) / float64(r.t.n)
		}
		fmt.Fprintf(w, "%s  %-42s %d/%d (%.1f%%), need %s\n", status, r.name, r.t.hit, r.t.n, pct, r.need)
		for _, f := range r.t.fails {
			fmt.Fprintf(w, "        miss: %s\n", f)
		}
	}
	if unjudged > 0 {
		fmt.Fprintf(w, "%d rows have no verdict: run calibrate again to retry them\n", unjudged)
	}
	if failed > 0 {
		return fmt.Errorf("calibration failed: %d of 3 sets", failed)
	}
	fmt.Fprintln(w, "calibration passed: the rubric may go to the agent runs")
	return nil
}

func calibrateCommand(args []string) error {
	var qPaths, probesPath, rubricPath, kind, model, out, pinsPath string
	var workers int
	var seed int64
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	fs.StringVar(&qPaths, "questions", filepath.Join("bench", "recover", "questions", "budgetron.json")+","+filepath.Join("bench", "recover", "questions", "deltadocs.json"), "comma-separated questions files; every rule of all of them is calibrated")
	fs.StringVar(&probesPath, "probes", filepath.Join("bench", "recover", "calibration-probes.json"), "DESIGN's probe table")
	fs.StringVar(&rubricPath, "rubric", filepath.Join("bench", "recover", "rubric.md"), "rubric file")
	fs.StringVar(&kind, "judge", "claude", "claude, or fake")
	fs.StringVar(&model, "model", "claude-sonnet-5-5", "judge model")
	fs.StringVar(&out, "o", "", "calibration CSV, also the resume state (default bench/recover/calibration-<judge>.csv)")
	fs.StringVar(&pinsPath, "pins", filepath.Join("bench", "recover", "PINS"), "pins: set (b)'s rules are chosen with SCHEDULE_SEED")
	fs.Int64Var(&seed, "seed", 0, "seed (default SCHEDULE_SEED from PINS)")
	fs.IntVar(&workers, "workers", 3, "parallel judge calls")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if seed == 0 {
		pins, err := loadPins(pinsPath)
		if err != nil {
			return err
		}
		seed, _ = strconv.ParseInt(pins["SCHEDULE_SEED"], 10, 64)
	}
	if out == "" {
		out = filepath.Join("bench", "recover", "calibration-"+kind+".csv")
	}
	var files []*questionFile
	for _, p := range splitList(qPaths) {
		f, err := loadQuestions(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		files = append(files, f)
	}
	probes, err := loadProbes(probesPath)
	if err != nil {
		return err
	}
	rows, err := buildCalibration(files, probes, seed)
	if err != nil {
		return err
	}
	var j judger
	rubric := ""
	switch kind {
	case "claude":
		bin, err := exec.LookPath("claude")
		if err != nil {
			return fmt.Errorf("claude: %w", err)
		}
		j = claudeJudge{model: model, bin: bin, timeout: 5 * time.Minute}
	case "fake":
		j = fakeJudge{}
	default:
		return fmt.Errorf("-judge wants claude or fake, got %q", kind)
	}
	if data, err := os.ReadFile(rubricPath); err == nil {
		rubric = string(data)
	} else if kind == "claude" {
		return fmt.Errorf("rubric: %w", err)
	}
	return calibrate(context.Background(), os.Stdout, out, rubric, j, rows, workers, seed)
}
