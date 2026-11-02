package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Rule classes. A rule is recoverable when the code arm gets it right, valid
// when the docs arm does; the census asks only about valid ones.
const (
	classRecoverable = "recoverable"  // valid and recoverable: the code says it too
	classNormative   = "normative"    // valid, not recoverable: only the docs say it
	classBad         = "bad_question" // not valid: even with the docs it is not answered
	classIncomplete  = "incomplete"   // fewer judged runs than reps in an arm
)

// ruleStat is one rule's counts in both arms.
type ruleStat struct {
	ID          string `json:"id"`
	Doc         string `json:"doc"`
	Predict     string `json:"predict"`
	Ignored     bool   `json:"ignored,omitempty"`
	CodeCorrect int    `json:"code_correct"`
	CodeRuns    int    `json:"code_runs"`
	DocsCorrect int    `json:"docs_correct"`
	DocsRuns    int    `json:"docs_runs"`
	Class       string `json:"class"`
}

// docStat is one doc's tally.
type docStat struct {
	Doc         string   `json:"doc"`
	Valid       int      `json:"valid"`
	Normative   int      `json:"normative"`
	Bad         int      `json:"bad_questions"`
	Share       float64  `json:"normative_share"`
	Low, High   float64  `json:"-"`
	CILow       float64  `json:"ci_low"`
	CIHigh      float64  `json:"ci_high"`
	NormativeID []string `json:"normative_ids"`
}

// tally is one group of rules' counts.
type tally struct {
	Rules       int      `json:"rules"`
	Valid       int      `json:"valid"`
	Recoverable int      `json:"recoverable"`
	Normative   int      `json:"normative"`
	Bad         int      `json:"bad_questions"`
	Incomplete  int      `json:"incomplete"`
	Share       float64  `json:"normative_share"`
	CILow       float64  `json:"ci_low"`
	CIHigh      float64  `json:"ci_high"`
	NormativeID []string `json:"normative_ids"`
}

func (t *tally) add(s ruleStat) {
	t.Rules++
	switch s.Class {
	case classBad:
		t.Bad++
	case classIncomplete:
		t.Incomplete++
	case classRecoverable:
		t.Valid++
		t.Recoverable++
	case classNormative:
		t.Valid++
		t.Normative++
		t.NormativeID = append(t.NormativeID, s.ID)
	}
}

func (t *tally) finish() {
	if t.NormativeID == nil {
		t.NormativeID = []string{}
	}
	if t.Valid > 0 {
		t.Share = float64(t.Normative) / float64(t.Valid)
	}
	t.CILow, t.CIHigh = wilson(t.Normative, t.Valid)
}

// repoSummary is normative-<repo>.json. Its primary fields cover the bound
// (non-ignored) rules; the ignored ones are their own row, outside the share.
type repoSummary struct {
	Ignored     *tally     `json:"ignored_rules,omitempty"`
	Repo        string     `json:"repo"`
	Commit      string     `json:"commit"`
	Reps        int        `json:"reps"`
	MinCorrect  int        `json:"min_correct"`
	Rules       []ruleStat `json:"rules"`
	Valid       int        `json:"valid"`
	Recoverable int        `json:"recoverable"`
	Normative   int        `json:"normative"`
	Bad         int        `json:"bad_questions"`
	Incomplete  int        `json:"incomplete"`
	Share       float64    `json:"normative_share"`
	CILow       float64    `json:"ci_low"`
	CIHigh      float64    `json:"ci_high"`
	NormativeID []string   `json:"normative_ids"`
	ByDoc       []docStat  `json:"by_doc"`
	Predictions struct {
		Agree    int `json:"agree"`
		Disagree int `json:"disagree"`
	} `json:"predictions"`
}

// wilson is the Wilson score interval for k of n at 95%.
func wilson(k, n int) (low, high float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.959964
	p, nn := float64(k)/float64(n), float64(n)
	denom := 1 + z*z/nn
	centre := (p + z*z/(2*nn)) / denom
	half := z * math.Sqrt(p*(1-p)/nn+z*z/(4*nn*nn)) / denom
	return math.Max(0, centre-half), math.Min(1, centre+half)
}

// classify applies the census rules: recoverable = code arm correct in at
// least minCorrect runs; valid = docs arm correct in at least minCorrect;
// normative = valid and not recoverable; bad question = not valid. A rule
// short of reps judged runs in either arm is incomplete and counts nowhere.
func classify(s ruleStat, reps, minCorrect int) string {
	if s.CodeRuns < reps || s.DocsRuns < reps {
		return classIncomplete
	}
	recoverable := s.CodeCorrect >= minCorrect
	valid := s.DocsCorrect >= minCorrect
	switch {
	case !valid:
		return classBad
	case recoverable:
		return classRecoverable
	}
	return classNormative
}

// summarize builds one repo's summary from its judged rows. A row counts as
// correct only on a CORRECT verdict; PARTIAL, WRONG and ABSTAIN do not. An
// unjudged row is not a run: it leaves its rule incomplete.
func summarize(rows []result, reps, minCorrect int) []repoSummary {
	byRepo := map[string]map[string]*ruleStat{}
	order := map[string][]string{}
	commits := map[string]string{}
	for _, r := range rows {
		if byRepo[r.Repo] == nil {
			byRepo[r.Repo] = map[string]*ruleStat{}
		}
		commits[r.Repo] = r.Commit
		s := byRepo[r.Repo][r.QuestionID]
		if s == nil {
			s = &ruleStat{ID: r.QuestionID, Doc: r.Doc, Predict: r.Predict, Ignored: r.Ignored}
			byRepo[r.Repo][r.QuestionID] = s
			order[r.Repo] = append(order[r.Repo], r.QuestionID)
		}
		if r.Verdict == "" {
			continue
		}
		ok := r.Verdict == "CORRECT"
		switch r.Arm {
		case armCode:
			s.CodeRuns++
			if ok {
				s.CodeCorrect++
			}
		case armDocs:
			s.DocsRuns++
			if ok {
				s.DocsCorrect++
			}
		}
	}
	var repos []string
	for repo := range byRepo {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	var out []repoSummary
	for _, repo := range repos {
		sum := repoSummary{Repo: repo, Commit: commits[repo], Reps: reps, MinCorrect: minCorrect}
		docs := map[string]*docStat{}
		for _, id := range order[repo] {
			s := byRepo[repo][id]
			s.Class = classify(*s, reps, minCorrect)
			sum.Rules = append(sum.Rules, *s)
			if s.Ignored {
				if sum.Ignored == nil {
					sum.Ignored = &tally{}
				}
				sum.Ignored.add(*s)
				continue
			}
			d := docs[s.Doc]
			if d == nil {
				d = &docStat{Doc: s.Doc, NormativeID: []string{}}
				docs[s.Doc] = d
			}
			switch s.Class {
			case classBad:
				sum.Bad++
				d.Bad++
			case classIncomplete:
				sum.Incomplete++
			case classRecoverable:
				sum.Valid++
				sum.Recoverable++
				d.Valid++
			case classNormative:
				sum.Valid++
				sum.Normative++
				d.Valid++
				d.Normative++
				sum.NormativeID = append(sum.NormativeID, s.ID)
				d.NormativeID = append(d.NormativeID, s.ID)
			}
			if s.Class == classRecoverable || s.Class == classNormative {
				if (s.Predict == "normative") == (s.Class == classNormative) {
					sum.Predictions.Agree++
				} else {
					sum.Predictions.Disagree++
				}
			}
		}
		if sum.Ignored != nil {
			sum.Ignored.finish()
		}
		if sum.NormativeID == nil {
			sum.NormativeID = []string{}
		}
		if sum.Valid > 0 {
			sum.Share = float64(sum.Normative) / float64(sum.Valid)
		}
		sum.CILow, sum.CIHigh = wilson(sum.Normative, sum.Valid)
		var names []string
		for name := range docs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			d := docs[name]
			if d.Valid > 0 {
				d.Share = float64(d.Normative) / float64(d.Valid)
			}
			d.CILow, d.CIHigh = wilson(d.Normative, d.Valid)
			sum.ByDoc = append(sum.ByDoc, *d)
		}
		out = append(out, sum)
	}
	return out
}

func percent(x float64) string { return fmt.Sprintf("%.0f%%", 100*x) }

func printSummary(w io.Writer, s repoSummary) {
	bound := 0
	for _, r := range s.Rules {
		if !r.Ignored {
			bound++
		}
	}
	fmt.Fprintf(w, "== %s @ %s: %d bound rules, %d reps per arm, correct means CORRECT in >= %d\n", s.Repo, short(s.Commit), bound, s.Reps, s.MinCorrect)
	fmt.Fprintf(w, "valid %d  (recoverable %d, normative %d)   bad questions %d   incomplete %d\n",
		s.Valid, s.Recoverable, s.Normative, s.Bad, s.Incomplete)
	fmt.Fprintf(w, "normative share: %d/%d = %s  (Wilson 95%% %s to %s)\n", s.Normative, s.Valid, percent(s.Share), percent(s.CILow), percent(s.CIHigh))
	if t := s.Ignored; t != nil {
		fmt.Fprintf(w, "ignored rules (own row, outside the share): %d rules, valid %d (recoverable %d, normative %d), bad %d, incomplete %d; normative %d/%d = %s (%s to %s): %s\n",
			t.Rules, t.Valid, t.Recoverable, t.Normative, t.Bad, t.Incomplete, t.Normative, t.Valid, percent(t.Share), percent(t.CILow), percent(t.CIHigh), strings.Join(t.NormativeID, " "))
	}
	fmt.Fprintf(w, "prediction agreement (valid bound rules): %d agree, %d disagree\n\n", s.Predictions.Agree, s.Predictions.Disagree)
	fmt.Fprintf(w, "%-12s %-10s %-12s %-13s  %s\n", "rule", "code", "docs", "class", "doc")
	for _, r := range s.Rules {
		mark := ""
		if r.Ignored {
			mark = "  (ignored)"
		}
		fmt.Fprintf(w, "%-12s %d/%-8d %d/%-10d %-13s  %s%s\n", r.ID, r.CodeCorrect, r.CodeRuns, r.DocsCorrect, r.DocsRuns, r.Class, r.Doc, mark)
	}
	fmt.Fprintf(w, "\nper doc (normative / valid):\n")
	for _, d := range s.ByDoc {
		fmt.Fprintf(w, "  %-40s %d/%d = %s  (%s to %s)  bad %d\n", d.Doc, d.Normative, d.Valid, percent(d.Share), percent(d.CILow), percent(d.CIHigh), d.Bad)
	}
	fmt.Fprintf(w, "\nnormative bound rules (%d): %s\n", len(s.NormativeID), strings.Join(s.NormativeID, " "))
	var bad, incomplete []string
	for _, r := range s.Rules {
		if r.Ignored {
			continue
		}
		switch r.Class {
		case classBad:
			bad = append(bad, r.ID)
		case classIncomplete:
			incomplete = append(incomplete, r.ID)
		}
	}
	fmt.Fprintf(w, "bad questions (%d): %s\n", len(bad), strings.Join(bad, " "))
	if len(incomplete) > 0 {
		fmt.Fprintf(w, "incomplete (%d, not counted; judge or run them): %s\n", len(incomplete), strings.Join(incomplete, " "))
	}
	fmt.Fprintln(w)
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func summaryCommand(args []string) error {
	var resultsPath, outDir string
	var reps, minCorrect int
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.StringVar(&resultsPath, "results", "", "judged results CSV")
	fs.StringVar(&outDir, "out-dir", "", "where normative-<repo>.json goes (default: next to the results)")
	fs.IntVar(&reps, "reps", 3, "runs per question and arm")
	fs.IntVar(&minCorrect, "min-correct", 2, "correct runs needed for a rule to count as recovered, or valid")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if resultsPath == "" {
		return fmt.Errorf("-results is required")
	}
	rows, err := readCSV(resultsPath)
	if err != nil {
		return err
	}
	if outDir == "" {
		outDir = filepath.Dir(resultsPath)
	}
	for _, s := range summarize(rows, reps, minCorrect) {
		printSummary(os.Stdout, s)
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		if err := writeFileAtomic(filepath.Join(outDir, "normative-"+s.Repo+".json"), append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}
