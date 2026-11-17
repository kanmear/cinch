// Command analyze reads leak replay results CSVs (bench/leak/results-<engine>.csv,
// and T4's private file beside it) and prints the pre-stated analysis of
// bench/leak/DESIGN.md § Analysis: per task and arm counts (delivered, leaked,
// safe, undelivered, rule written), the primary contrast (safe rate, arm 3
// against arm 1, pooled over T1-T3 and stratified by task, one-sided
// Cochran-Mantel-Haenszel plain, continuity-corrected and exact), the look
// verdict, T1 on its own and T2-T3 pooled, T4 and arm 2 descriptively, the
// secondary measures, and with -pilot the pilot rule's keep conditions and caps.
//
//	go run ./bench/leak/analyze -look 1 bench/leak/results-pi.csv bench/leak/private/results-pi.csv
//	go run ./bench/leak/analyze -pilot bench/leak/pilot/results-pi.csv bench/leak/private/pilot-results-pi.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// The registered looks (DESIGN § Analysis): the plain CMH p must be below the
// look's alpha and the pooled difference at least lookDiff points.
const (
	lookDiff = 15.0
)

var lookAlpha = map[int]float64{1: 0.005, 2: 0.048}

// lookN is the per-task, per-arm run count a look is registered at.
var lookN = map[int]int{1: 8, 2: 15}

// primaryTasks are the tasks of the primary; T1 is the in-example task, T2-T3
// the generalization test; T4 is descriptive.
var primaryTasks = []string{"T1", "T2", "T3"}

// run is the final attempt of one run_id.
type run struct {
	ID, Task                        string
	Arm, Attempt                    int
	Pilot                           bool
	Status                          string
	TimedOut, Committed, Bypass     bool
	Delivered, Leaked, Safe         bool
	Outcome                         string
	Structural, Written, WrittenSen string
	Bound                           string
	HookBlocks                      int
	Turns, WallMin, OutTokens       float64
	InTokens, CapMin                float64
}

func main() {
	look := flag.Int("look", 1, "which registered look to evaluate: 1 (after 8 runs per task in arms 1 and 3) or 2 (after 15)")
	pilot := flag.Bool("pilot", false, "evaluate the pilot rule on the pilot rows (DESIGN § Pilot and cap) instead of the study")
	flag.Parse()
	if flag.NArg() < 1 || (*look != 1 && *look != 2) {
		fmt.Fprintln(os.Stderr, "usage: analyze [-look 1|2] [-pilot] results.csv [more.csv ...]")
		os.Exit(2)
	}
	var all []run
	for _, path := range flag.Args() {
		runs, err := readRuns(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "analyze:", err)
			os.Exit(1)
		}
		all = append(all, runs...)
	}
	var picked []run
	for _, r := range all {
		if r.Pilot == *pilot {
			picked = append(picked, r)
		}
	}
	sort.Slice(picked, func(a, b int) bool { return picked[a].ID < picked[b].ID })
	if *pilot {
		reportPilot(os.Stdout, strings.Join(flag.Args(), ", "), picked)
		return
	}
	report(os.Stdout, strings.Join(flag.Args(), ", "), picked, *look)
}

// readRuns parses a results CSV and keeps the final attempt per run_id.
func readRuns(path string) ([]run, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comment = '#'
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: empty", path)
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	for _, name := range []string{"run_id", "task", "arm", "attempt", "pilot", "status", "outcome", "delivered", "leaked", "safe",
		"timed_out", "committed", "bypass", "hook_blocks", "turns", "wall_s", "output_tokens", "rule_structural", "rule_written"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("%s: no %q column", path, name)
		}
	}
	final := map[string]run{}
	for _, row := range rows[1:] {
		get := func(name string) string {
			if i, ok := col[name]; ok && i < len(row) {
				return row[i]
			}
			return ""
		}
		num := func(name string) float64 {
			v, _ := strconv.ParseFloat(get(name), 64)
			return v
		}
		r := run{
			ID: get("run_id"), Task: get("task"), Arm: int(num("arm")), Attempt: int(num("attempt")),
			Pilot: get("pilot") == "true", Status: get("status"), Outcome: get("outcome"),
			TimedOut: get("timed_out") == "true", Committed: get("committed") == "true", Bypass: get("bypass") == "true",
			Delivered: get("delivered") == "true", Leaked: get("leaked") == "true", Safe: get("safe") == "true",
			Structural: get("rule_structural"), Written: get("rule_written"), WrittenSen: get("rule_written_sens"), Bound: get("rule_bound"),
			HookBlocks: int(num("hook_blocks")), Turns: num("turns"), WallMin: num("wall_s") / 60,
			OutTokens: num("output_tokens"), InTokens: num("input_tokens"), CapMin: num("cap_min"),
		}
		key := get("pilot") + "/" + r.ID
		if prev, ok := final[key]; !ok || r.Attempt >= prev.Attempt {
			final[key] = r
		}
	}
	var runs []run
	for _, r := range final {
		runs = append(runs, r)
	}
	sort.Slice(runs, func(a, b int) bool { return runs[a].ID < runs[b].ID })
	return runs, nil
}

// stratum is one task's 2x2 table: successes and runs in the treatment arm (A)
// and the control arm (B).
type stratum struct {
	Task           string
	XA, NA, XB, NB int
}

func (s stratum) n() int { return s.NA + s.NB }
func (s stratum) m() int { return s.XA + s.XB }

// cmh returns the one-sided CMH z (A above B) without and with the continuity
// correction, and their p values.
func cmh(strata []stratum) (z, p, zCC, pCC float64) {
	var dev, variance float64
	for _, s := range strata {
		n, m := float64(s.n()), float64(s.m())
		if s.n() < 2 {
			continue
		}
		dev += float64(s.XA) - float64(s.NA)*m/n
		variance += float64(s.NA) * float64(s.NB) * m * (n - m) / (n * n * (n - 1))
	}
	if variance == 0 {
		return 0, 0.5, 0, 0.5
	}
	z = dev / math.Sqrt(variance)
	zCC = (dev - 0.5) / math.Sqrt(variance)
	return z, upper(z), zCC, upper(zCC)
}

// upper is the standard normal upper tail.
func upper(z float64) float64 { return 0.5 * math.Erfc(z/math.Sqrt2) }

// choose is the binomial coefficient as a float.
func choose(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	out := 1.0
	for i := 1; i <= k; i++ {
		out = out * float64(n-k+i) / float64(i)
	}
	return out
}

// exact is the one-sided exact p of the stratified test: the probability, under
// the null with every stratum's margins fixed, that arm A has at least as many
// successes as observed. It convolves the per-task hypergeometric laws. With
// one stratum it is Fisher's exact test.
func exact(strata []stratum) float64 {
	dist := []float64{1}
	observed := 0
	for _, s := range strata {
		if s.n() == 0 {
			continue
		}
		observed += s.XA
		lo, hi := max(0, s.m()-s.NB), min(s.NA, s.m())
		next := make([]float64, len(dist)+hi)
		for k := lo; k <= hi; k++ {
			pk := choose(s.NA, k) * choose(s.NB, s.m()-k) / choose(s.n(), s.m())
			for i, v := range dist {
				next[i+k] += v * pk
			}
		}
		dist = next
	}
	p := 0.0
	for i := observed; i < len(dist); i++ {
		p += dist[i]
	}
	return math.Min(p, 1)
}

func sums(strata []stratum) (xa, na, xb, nb int) {
	for _, s := range strata {
		xa, na, xb, nb = xa+s.XA, na+s.NA, xb+s.XB, nb+s.NB
	}
	return
}

// diffPoints is the pooled A minus B rate, in percentage points.
func diffPoints(strata []stratum) float64 {
	xa, na, xb, nb := sums(strata)
	if na == 0 || nb == 0 {
		return 0
	}
	return 100 * (float64(xa)/float64(na) - float64(xb)/float64(nb))
}

// strataFor builds one stratum per task of the list for treatment arm a against
// control arm b, counting runs for which ok is true. Invalid runs are not in
// the strata (they are counted separately and rerun).
func strataFor(runs []run, tasks []string, a, b int, ok func(run) bool) []stratum {
	var out []stratum
	for _, task := range tasks {
		s := stratum{Task: task}
		for _, r := range runs {
			if r.Task != task || r.Status == "invalid" {
				continue
			}
			switch r.Arm {
			case a:
				s.NA++
				s.XA += b2i(ok(r))
			case b:
				s.NB++
				s.XB += b2i(ok(r))
			}
		}
		if s.NA > 0 && s.NB > 0 {
			out = append(out, s)
		}
	}
	return out
}

func isSafe(r run) bool { return r.Safe }

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	if len(v)%2 == 1 {
		return v[len(v)/2]
	}
	return (v[len(v)/2-1] + v[len(v)/2]) / 2
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func frac(a, n int) string { return fmt.Sprintf("%d/%d", a, n) }

func pct(a, n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(a)/float64(n))
}

func tasksOf(runs []run) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range runs {
		if !seen[r.Task] {
			seen[r.Task] = true
			out = append(out, r.Task)
		}
	}
	sort.Strings(out)
	return out
}

func cell(runs []run, task string, arm int) []run {
	var out []run
	for _, r := range runs {
		if r.Task == task && r.Arm == arm {
			out = append(out, r)
		}
	}
	return out
}

// counts is one cell's outcome tally. Invalid runs are listed apart: they are
// rerun, and a final invalid attempt counts in no outcome.
type counts struct {
	N, Invalid, Delivered, Leaked, Safe, Undelivered  int
	Structural, Written, WrittenSens, Judged, Pending int
	RuleApplies                                       bool
}

func tally(runs []run) counts {
	var c counts
	for _, r := range runs {
		if r.Status == "invalid" {
			c.Invalid++
			continue
		}
		c.N++
		c.Delivered += b2i(r.Delivered)
		c.Leaked += b2i(r.Leaked)
		c.Safe += b2i(r.Safe)
		c.Undelivered += b2i(!r.Delivered)
		if r.Arm == 2 {
			continue
		}
		switch {
		case r.Written != "" && r.Written != "n/a":
			c.RuleApplies = true
			c.Structural += b2i(r.Structural == "pass")
			switch r.Written {
			case "yes":
				c.Written++
				c.Judged++
			case "no":
				c.Judged++
			case "pending":
				c.Pending++
			}
			c.WrittenSens += b2i(r.WrittenSen == "yes")
		case r.Bound != "" && r.Bound != "n/a":
			c.RuleApplies = true
			c.Structural += b2i(r.Bound == "yes" || r.Bound == "pending")
			switch r.Bound {
			case "yes":
				c.Written++
				c.Judged++
				c.WrittenSens++
			case "no":
				c.Judged++
			case "pending":
				c.Pending++
			}
		}
	}
	return c
}

func report(w io.Writer, path string, runs []run, look int) {
	fmt.Fprintf(w, "# %s: %d runs (final attempt per run_id; pilot rows excluded)\n", path, len(runs))
	if len(runs) == 0 {
		return
	}
	tasks := tasksOf(runs)

	fmt.Fprintln(w, "\n## Per task and arm")
	fmt.Fprintln(w, "delivered = every F test passes; leaked/safe are among delivered; rule written is for arms 1 and 3 of T1-T3, rule bound for T4 (structural/judged-yes/judged of N; sens counts PARTIAL)")
	fmt.Fprintf(w, "%-5s %-3s %-4s %-9s %-9s %-9s %-12s %-11s %-12s %-5s\n", "task", "arm", "n", "delivered", "leaked", "safe", "undelivered", "rule_struct", "rule_judged", "sens")
	for _, task := range tasks {
		for arm := 1; arm <= 3; arm++ {
			c := tally(cell(runs, task, arm))
			if c.N+c.Invalid == 0 {
				continue
			}
			n := c.N
			rs, rj, sens := "n/a", "n/a", "n/a"
			if c.RuleApplies {
				rs = fmt.Sprintf("%d/%d", c.Structural, n)
				rj = fmt.Sprintf("%d/%d", c.Written, c.Judged)
				if c.Pending > 0 {
					rj += fmt.Sprintf(" +%dp", c.Pending)
				}
				sens = frac(c.WrittenSens, n)
			}
			note := ""
			if c.Invalid > 0 {
				note = fmt.Sprintf("  (%d invalid, not counted)", c.Invalid)
			}
			fmt.Fprintf(w, "%-5s %-3d %-4d %-9s %-9s %-9s %-12s %-11s %-12s %-5s%s\n", task, arm, n,
				frac(c.Delivered, n), leakedLine(c), frac(c.Safe, n), frac(c.Undelivered, n), rs, rj, sens, note)
		}
	}
	fmt.Fprintln(w, "(leaked is k/delivered; \"p\" after a judged count is candidates still waiting for the judge)")

	var present []string
	for _, t := range primaryTasks {
		if contains(tasks, t) {
			present = append(present, t)
		}
	}
	st := strataFor(runs, present, 3, 1, isSafe)
	fmt.Fprintf(w, "\n## Primary: safe rate, arm 3 against arm 1, %s pooled, stratified by task\n", strings.Join(present, "-"))
	if len(st) == 0 {
		fmt.Fprintln(w, "no task has runs in both arms 1 and 3")
	} else {
		contrast(w, "", st)
		alpha := lookAlpha[look]
		_, p, _, pCC := cmh(st)
		diff := diffPoints(st)
		pExact := exact(st)
		verdict := func(p float64) bool { return p < alpha && diff >= lookDiff }
		fmt.Fprintf(w, "\nlook %d (plain CMH p < %.3f and pooled difference >= %.0f points): ", look, alpha, lookDiff)
		if verdict(p) {
			fmt.Fprintln(w, "arm 3 beats arm 1")
		} else {
			fmt.Fprintln(w, "not detected at this N")
		}
		fmt.Fprintf(w, "  by the corrected CMH: %t; by the exact p: %t (the registered statistic is the plain CMH)\n", verdict(pCC), verdict(pExact))
		want := lookN[look]
		for _, s := range st {
			if s.NA != want || s.NB != want {
				fmt.Fprintf(w, "  note: %s has %d runs in arm 3 and %d in arm 1; look %d is registered at %d per cell\n", s.Task, s.NA, s.NB, look, want)
			}
		}
		if look == 2 {
			fmt.Fprintln(w, "  (look 2 counts all 15 runs per cell; look 1's runs are among them, never re-tested separately)")
		}
		capBound(w, runs, present)

		fmt.Fprintln(w, "\n## Key secondary and the in-example split")
		if t1 := strataFor(runs, []string{"T1"}, 3, 1, isSafe); len(t1) > 0 {
			contrast(w, "T1 (in the step's own example) ", t1)
		}
		var rest []string
		for _, t := range present {
			if t != "T1" {
				rest = append(rest, t)
			}
		}
		if st23 := strataFor(runs, rest, 3, 1, isSafe); len(st23) > 0 {
			contrast(w, strings.Join(rest, "-")+" pooled (out of the example) ", st23)
			d := diffPoints(st23)
			_, p1, _, _ := cmh(st)
			if p1 < lookAlpha[look] && diffPoints(st) >= lookDiff && d < lookDiff {
				fmt.Fprintf(w, "reading rule: the primary wins and %s's pooled difference is %+.1f points (< %.0f): the step prevents the bug its own example names; generalization not shown\n", strings.Join(rest, "-"), d, lookDiff)
			}
		}
		fmt.Fprintln(w, "\nper-task difference (arm 3 minus arm 1, points):")
		for _, s := range st {
			fmt.Fprintf(w, "  %-4s arm 3 %s  arm 1 %s  %+.0f\n", s.Task, frac(s.XA, s.NA), frac(s.XB, s.NB), 100*(rate(s.XA, s.NA)-rate(s.XB, s.NB)))
		}
	}

	fmt.Fprintln(w, "\n## Leaked among delivered (plan 24's headline line), by arm")
	for _, task := range tasks {
		var parts []string
		for arm := 1; arm <= 3; arm++ {
			c := tally(cell(runs, task, arm))
			if c.N == 0 {
				continue
			}
			parts = append(parts, fmt.Sprintf("arm %d leaked %d/%d (delivered %d/%d)", arm, c.Leaked, c.Delivered, c.Delivered, c.N))
		}
		fmt.Fprintf(w, "%-4s %s\n", task, strings.Join(parts, "; "))
	}

	fmt.Fprintln(w, "\n## Within arm 3: safe with and without a rule written (descriptive)")
	for _, task := range tasks {
		with, without := []run{}, []run{}
		for _, r := range cell(runs, task, 3) {
			if r.Status == "invalid" {
				continue
			}
			switch r.Written {
			case "yes":
				with = append(with, r)
			case "no":
				without = append(without, r)
			}
		}
		if len(with)+len(without) == 0 {
			continue
		}
		fmt.Fprintf(w, "%-4s rule written: safe %s; no rule: safe %s\n", task, frac(countSafe(with), len(with)), frac(countSafe(without), len(without)))
	}

	if t4 := cellsOf(runs, "T4"); len(t4) > 0 {
		fmt.Fprintln(w, "\n## T4 (descriptive; the rule exists, so the step cannot add it)")
		contrastRuns(w, "T4 ", runs, []string{"T4"}, 3, 1)
		for _, arm := range []int{1, 3} {
			c := tally(cell(runs, "T4", arm))
			if c.N > 0 {
				fmt.Fprintf(w, "  arm %d: rule bound %d/%d (pending %d)\n", arm, c.Written, c.N, c.Pending)
			}
		}
	}

	fmt.Fprintln(w, "\n## Arm 2 (rule present), descriptive: against arm 1 per task, one-sided exact p (arm 2 safer)")
	any2 := false
	for _, task := range tasks {
		s2 := strataFor(runs, []string{task}, 2, 1, isSafe)
		if len(s2) == 0 {
			continue
		}
		any2 = true
		s := s2[0]
		c := tally(cell(runs, task, 2))
		fmt.Fprintf(w, "%-4s arm 2 safe %s, leaked %d/%d, delivered %s   arm 1 safe %s   p = %.4f\n", task, frac(s.XA, s.NA), c.Leaked, c.Delivered, frac(c.Delivered, c.N), frac(s.XB, s.NB), exact(s2))
	}
	if !any2 {
		fmt.Fprintln(w, "no arm 2 runs")
	}

	fmt.Fprintln(w, "\n## Secondary, per arm (all tasks)")
	fmt.Fprintf(w, "%-3s %-4s %-9s %-7s %-9s %-11s %-7s %-9s %-10s %-11s\n",
		"arm", "n", "committed", "bypass", "timed_out", "hook_blocks", "turns", "wall_min", "in_tokens", "out_tokens")
	for arm := 1; arm <= 3; arm++ {
		var n, com, byp, to, blocks int
		var turns, wall, in, out []float64
		for _, r := range runs {
			if r.Arm != arm || r.Status == "invalid" {
				continue
			}
			n++
			com += b2i(r.Committed)
			byp += b2i(r.Bypass)
			to += b2i(r.TimedOut)
			blocks += r.HookBlocks
			turns, wall, in, out = append(turns, r.Turns), append(wall, r.WallMin), append(in, r.InTokens), append(out, r.OutTokens)
		}
		if n == 0 {
			continue
		}
		fmt.Fprintf(w, "%-3d %-4d %-9s %-7d %-9d %-11d %-7.0f %-9.1f %-10.0f %-11.0f\n", arm, n, frac(com, n), byp, to, blocks, median(turns), median(wall), median(in), median(out))
	}
	fmt.Fprintln(w, "(bypass, timed_out and hook_blocks are totals over runs; turns, wall_min and tokens are medians)")
}

func leakedLine(c counts) string { return frac(c.Leaked, c.Delivered) }

func countSafe(runs []run) int {
	n := 0
	for _, r := range runs {
		n += b2i(r.Safe)
	}
	return n
}

func cellsOf(runs []run, task string) []run {
	var out []run
	for _, r := range runs {
		if r.Task == task {
			out = append(out, r)
		}
	}
	return out
}

func rate(x, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(x) / float64(n)
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// contrast prints one pooled arm 3 against arm 1 comparison with its tests.
func contrast(w io.Writer, label string, st []stratum) {
	xa, na, xb, nb := sums(st)
	z, p, zCC, pCC := cmh(st)
	fmt.Fprintf(w, "%sarm 3: %d/%d (%.1f%%)   arm 1: %d/%d (%.1f%%)   pooled difference: %+.1f points\n", label,
		xa, na, 100*rate(xa, na), xb, nb, 100*rate(xb, nb), diffPoints(st))
	fmt.Fprintf(w, "  one-sided CMH:                   z = %+.3f  p = %.4f\n", z, p)
	fmt.Fprintf(w, "  one-sided CMH, continuity-corr.: z = %+.3f  p = %.4f\n", zCC, pCC)
	fmt.Fprintf(w, "  one-sided exact (stratified):               p = %.4f\n", exact(st))
}

func contrastRuns(w io.Writer, label string, runs []run, tasks []string, a, b int) {
	if st := strataFor(runs, tasks, a, b, isSafe); len(st) > 0 {
		contrast(w, label, st)
	}
}

// capBound flags tasks where more than a quarter of either compared arm's runs
// timed out (DESIGN § Pilot and cap, during the study) and prints the primary
// without them.
func capBound(w io.Writer, runs []run, tasks []string) {
	var bound, rest []string
	for _, task := range tasks {
		flagged := false
		for _, arm := range []int{1, 3} {
			var n, to int
			for _, r := range cell(runs, task, arm) {
				if r.Status == "invalid" {
					continue
				}
				n++
				to += b2i(r.TimedOut)
			}
			if n > 0 && float64(to)/float64(n) > 0.25 {
				flagged = true
			}
		}
		if flagged {
			bound = append(bound, task)
		} else {
			rest = append(rest, task)
		}
	}
	if len(bound) == 0 {
		fmt.Fprintln(w, "cap-bound tasks (more than 25% timed out in arm 1 or arm 3): none")
		return
	}
	fmt.Fprintf(w, "cap-bound tasks (more than 25%% timed out in arm 1 or arm 3): %s\n", strings.Join(bound, ", "))
	if st := strataFor(runs, rest, 3, 1, isSafe); len(st) > 0 {
		contrast(w, "without them: ", st)
	}
	committed := func(r run) bool { return r.Safe && r.Committed }
	if st := strataFor(committedOnly(runs), tasks, 3, 1, committed); len(st) > 0 {
		contrast(w, "conditioned on committed runs: ", st)
	}
}

func committedOnly(runs []run) []run {
	var out []run
	for _, r := range runs {
		if r.Committed {
			out = append(out, r)
		}
	}
	return out
}

// ----- the pilot rule -----

// cap is DESIGN's cap per task: the larger of 30 minutes and 1.5 x the slowest
// committing pilot (arm 1 or arm 3), rounded up to 5 minutes, at most 75.
func capFor(slowestMin float64) int {
	c := math.Max(30, 1.5*slowestMin)
	c = math.Ceil(c/5) * 5
	return int(math.Min(c, 75))
}

type pilotVerdict struct {
	Task                             string
	Arm1, Arm1Committed, Arm1Safe    int
	Arm3, Arm3Committed              int
	Slowest                          float64
	Keep                             bool
	Ceiling, NeedSecondArm3, Dropped bool
	CapMin                           int
	Reasons                          []string
}

// evalPilot applies DESIGN's keep rule to one task's pilot runs. A pilot
// "commits before the cap" when it committed (a run that timed out after
// committing counts, at its wall time). Calibration (a) is checked by the
// calibration runs, not by this file.
func evalPilot(task string, runs []run) pilotVerdict {
	v := pilotVerdict{Task: task, Keep: true}
	var wall []float64
	for _, r := range runs {
		if r.Task != task || r.Status == "invalid" {
			continue
		}
		switch r.Arm {
		case 1:
			v.Arm1++
			v.Arm1Committed += b2i(r.Committed)
			v.Arm1Safe += b2i(r.Safe)
		case 3:
			v.Arm3++
			v.Arm3Committed += b2i(r.Committed)
		}
		if r.Committed {
			wall = append(wall, r.WallMin)
		}
	}
	for _, m := range wall {
		v.Slowest = math.Max(v.Slowest, m)
	}
	if v.Arm1 < 3 {
		v.Reasons = append(v.Reasons, fmt.Sprintf("only %d of 3 arm-1 pilots have run", v.Arm1))
		v.Keep = false
	} else if v.Arm1Committed < 2 {
		v.Reasons = append(v.Reasons, fmt.Sprintf("(b) fails: %d of %d arm-1 pilots committed (need 2 of 3)", v.Arm1Committed, v.Arm1))
		v.Keep = false
	}
	switch {
	case v.Arm3 == 0:
		v.Reasons = append(v.Reasons, "no arm-3 pilot has run")
		v.Keep = false
	case v.Arm3Committed == 0 && v.Arm3 == 1:
		v.Reasons = append(v.Reasons, "(c) the arm-3 pilot did not commit: run one more arm-3 pilot")
		v.NeedSecondArm3, v.Keep = true, false
	case v.Arm3Committed == 0:
		v.Reasons = append(v.Reasons, "(c) fails: no arm-3 pilot committed, even the second one: the task is dropped")
		v.Dropped, v.Keep = true, false
	}
	if v.Arm1 >= 3 && v.Arm1Safe == v.Arm1 {
		v.Reasons = append(v.Reasons, "(d) fails: all 3 arm-1 pilots are safe deliveries: the task leaves the primary, runs at N = 5 in arm 1 only")
		v.Ceiling, v.Keep = true, false
	}
	if len(wall) > 0 || v.Keep {
		v.CapMin = capFor(v.Slowest)
	}
	return v
}

func reportPilot(w io.Writer, path string, runs []run) {
	fmt.Fprintf(w, "# %s: %d pilot runs (final attempt per run_id)\n", path, len(runs))
	fmt.Fprintln(w, "pilot rule (DESIGN § Pilot and cap): keep a task if (a) calibration passed [checked by the calibration runs, assumed here], (b) >= 2 of 3 arm-1 pilots commit, (c) the arm-3 pilot commits (else one more; dropped if that fails too), (d) not all 3 arm-1 pilots are safe deliveries")
	fmt.Fprintln(w, "cap C = max(30, 1.5 x slowest committing pilot) minutes, rounded up to 5, at most 75")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-5s %-9s %-11s %-9s %-11s %-9s %-8s %-7s\n", "task", "arm1 n", "arm1 commit", "arm1 safe", "arm3 n", "arm3 commit", "slowest", "cap")
	var verdicts []pilotVerdict
	var kept []string
	for _, task := range []string{"T1", "T2", "T3", "T4"} {
		if len(cellsOf(runs, task)) == 0 {
			continue
		}
		v := evalPilot(task, runs)
		verdicts = append(verdicts, v)
		cp := "-"
		if v.CapMin > 0 {
			cp = fmt.Sprintf("%d", v.CapMin)
		}
		fmt.Fprintf(w, "%-5s %-9d %-11d %-9d %-11d %-9d %-8.1f %-7s\n", task, v.Arm1, v.Arm1Committed, v.Arm1Safe, v.Arm3, v.Arm3Committed, v.Slowest, cp)
		if v.Keep {
			kept = append(kept, task)
		}
	}
	fmt.Fprintln(w)
	for _, v := range verdicts {
		if v.Keep {
			fmt.Fprintf(w, "%s: KEEP, cap %d minutes\n", v.Task, v.CapMin)
			continue
		}
		fmt.Fprintf(w, "%s: not kept: %s\n", v.Task, strings.Join(v.Reasons, "; "))
		if v.Ceiling {
			fmt.Fprintf(w, "    cap if run at N = 5 in arm 1 only: %d minutes\n", v.CapMin)
		}
	}
	primary := 0
	for _, k := range kept {
		if k != "T4" {
			primary++
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "KEPT_TASKS=%s\n", strings.Join(kept, ","))
	for _, v := range verdicts {
		if v.Keep {
			fmt.Fprintf(w, "CAP_%s=%d\n", v.Task, v.CapMin)
		}
	}
	if primary < 2 {
		fmt.Fprintf(w, "fewer than two of T1-T3 survive (%d): the primary is not run; the owner decides whether to replace tasks under a new pilot tag\n", primary)
	}
}
