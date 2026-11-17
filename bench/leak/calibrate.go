package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// The rule judge's calibration (DESIGN § Calibration): per task, the real rule
// and a paraphrase must be STATES; the near-miss rules in each NOTES must be
// ABSENT; the listed PARTIAL probe must be PARTIAL; the references' rules must
// be STATES. Every probe must get its expected verdict, for each judge.

// probe is one candidate rule text and the verdict the judge must give it.
type probe struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Expected string `json:"expected"`
}

type probeFile struct {
	Comment string  `json:"comment"`
	Probes  []probe `json:"probes"`
}

// loadProbes reads tasks/<T>/judge-probes.json (T4's under private/).
func loadProbes(path string) ([]probe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f probeFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("probes %s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, p := range f.Probes {
		if p.ID == "" || strings.TrimSpace(p.Rule) == "" || !contains(verdicts, p.Expected) {
			return nil, fmt.Errorf("probes %s: %q: id, rule and an expected verdict (%s) are required", path, p.ID, strings.Join(verdicts, ", "))
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("probes %s: duplicate id %q", path, p.ID)
		}
		seen[p.ID] = true
	}
	if len(f.Probes) == 0 {
		return nil, fmt.Errorf("probes %s: none", path)
	}
	return f.Probes, nil
}

// calRow is one probe and, once judged, its verdict.
type calRow struct {
	Task, ID, Expected, Verdict, Why, Judge, Err string
	probe                                        probe
	invariant                                    string
}

func calibrateMain(args []string) error {
	var c config
	var kind, model, rubricPath, tasks, out string
	var workers int
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	commonFlags(fs, &c)
	fs.StringVar(&kind, "judge", "claude", "claude, pi or fake (a plumbing check: it cannot pass the near-miss probes)")
	fs.StringVar(&model, "model", "", "judge model")
	fs.StringVar(&rubricPath, "rubric", filepath.Join("bench", "leak", "rubric.md"), "rubric file with {{invariant}} and {{rule}}")
	fs.StringVar(&tasks, "tasks", "", "comma-separated tasks (default all with a probes file)")
	fs.StringVar(&out, "o", "", "CSV of every probe's verdict, also the resume state (default <work>/csv/judge-calibration-<judge>.csv; it holds rule text, so keep it private)")
	fs.IntVar(&workers, "workers", 3, "parallel judge calls")
	fs.StringVar(&c.piNode, "pi-node", "", "node binary that runs pi")
	fs.StringVar(&c.piCLI, "pi-cli", "", "pi's cli.js")
	if err := fs.Parse(args); err != nil {
		return err
	}
	h, err := newHarness(c)
	if err != nil {
		return err
	}
	j, err := makeJudge(kind, "", model, h.pins, c.piNode, c.piCLI)
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
	taskList := splitList(tasks)
	if len(taskList) == 0 {
		taskList = canonicalTasks
	}
	var rows []*calRow
	for _, task := range taskList {
		dir, _, err := taskDir(h.leakDir, task)
		if err != nil {
			continue
		}
		path := filepath.Join(dir, "judge-probes.json")
		if _, err := os.Stat(path); err != nil {
			logf("%s: no judge-probes.json, skipped", task)
			continue
		}
		probes, err := loadProbes(path)
		if err != nil {
			return err
		}
		inv, err := invariantOf(dir)
		if err != nil {
			return err
		}
		for _, p := range probes {
			rows = append(rows, &calRow{Task: task, ID: p.ID, Expected: p.Expected, probe: p, invariant: inv})
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no probes found")
	}
	if out == "" {
		out = filepath.Join(h.work, "csv", "judge-calibration-"+strings.NewReplacer(":", "-", "/", "-").Replace(j.name())+".csv")
	}
	if err := resumeCal(out, rows); err != nil {
		return err
	}
	return calibrate(context.Background(), rows, rubric, j, workers, out)
}

// calibrate judges every probe without a verdict, saves the CSV after each call,
// and reports PASS or FAIL per task.
func calibrate(ctx context.Context, rows []*calRow, rubric string, j judger, workers int, out string) error {
	var mu sync.Mutex
	save := func() error { return writeCal(out, rows) }
	if workers < 1 {
		workers = 1
	}
	queue := make(chan *calRow)
	var wg sync.WaitGroup
	var saveErr error
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range queue {
				verdict, why, err := judgeOne(ctx, rubric, j, judgeInput{Invariant: r.invariant, Rule: r.probe.Rule})
				mu.Lock()
				if err != nil {
					r.Err = err.Error()
				} else {
					r.Verdict, r.Why, r.Judge, r.Err = verdict, why, j.name(), ""
				}
				if err := save(); err != nil && saveErr == nil {
					saveErr = err
				}
				mu.Unlock()
			}
		}()
	}
	for _, r := range rows {
		if r.Verdict == "" {
			queue <- r
		}
	}
	close(queue)
	wg.Wait()
	if saveErr != nil {
		return saveErr
	}
	return reportCal(rows)
}

// reportCal prints PASS or FAIL per task and names every miss; it fails when
// any probe missed or has no verdict.
func reportCal(rows []*calRow) error {
	byTask := map[string][]*calRow{}
	for _, r := range rows {
		byTask[r.Task] = append(byTask[r.Task], r)
	}
	tasks := make([]string, 0, len(byTask))
	for t := range byTask {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)
	failed := 0
	for _, task := range tasks {
		var misses []string
		for _, r := range byTask[task] {
			switch {
			case r.Verdict == "":
				misses = append(misses, fmt.Sprintf("%s: no verdict (%s)", r.ID, r.Err))
			case r.Verdict != r.Expected:
				misses = append(misses, fmt.Sprintf("%s: expected %s, got %s", r.ID, r.Expected, r.Verdict))
			}
		}
		status := "PASS"
		if len(misses) > 0 {
			status = "FAIL"
			failed += len(misses)
		}
		fmt.Printf("%s  %s  %d probes\n", status, task, len(byTask[task]))
		for _, m := range misses {
			fmt.Printf("      %s\n", m)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d probe(s) missed", failed)
	}
	return nil
}

var calHeader = []string{"task", "id", "expected", "verdict", "justification", "judge", "judge_error"}

func writeCal(path string, rows []*calRow) error {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write(calHeader)
	for _, r := range rows {
		w.Write([]string{r.Task, r.ID, r.Expected, r.Verdict, r.Why, r.Judge, r.Err})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o600)
}

// resumeCal fills rows' verdicts from an earlier CSV, so a rerun only retries
// the probes without one.
func resumeCal(path string, rows []*calRow) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}
	prior := map[string][]string{}
	for _, rec := range records[1:] {
		if len(rec) >= len(calHeader) {
			prior[rec[0]+"/"+rec[1]] = rec
		}
	}
	for _, r := range rows {
		if rec, ok := prior[r.Task+"/"+r.ID]; ok && rec[3] != "" && rec[2] == r.Expected {
			r.Verdict, r.Why, r.Judge = rec[3], rec[4], rec[5]
		}
	}
	return nil
}
