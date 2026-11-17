// Command analyze reads an upkeep results CSV (bench/upkeep/results-<engine>.csv)
// and prints the pre-stated analysis of DESIGN.md § Analysis: upkeep counts per
// task and arm, the pooled arm-3 against arm-2 difference, the one-sided
// Cochran-Mantel-Haenszel test stratified by task (plain, with continuity
// correction, and exact), the ceiling tasks, the look-1 verdict, and the
// secondary measures. With -transcripts it also reads each run's
// transcript.jsonl and reports how the agents used cinch and the docs.
//
//	go run ./bench/upkeep/analyze bench/upkeep/results-pi.csv
//	go run ./bench/upkeep/analyze -transcripts ~/.cache/cinch-upkeep/runs/pi bench/upkeep/results-pi.csv
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Look 1 of DESIGN.md § Analysis: p below lookAlpha and at least lookDiff points.
const (
	lookAlpha = 0.005
	lookDiff  = 15.0
)

// run is the final attempt of one run_id.
type run struct {
	ID        string
	Task      string
	Arm       int
	Attempt   int
	TimedOut  bool
	Committed bool
	Bypass    bool
	Behavior  string
	Corpus    string
	Upkeep    string
	Suite     string
	Blocks    int
	Turns     float64
	WallMin   float64
	OutTokens float64
}

func main() {
	transcripts := flag.String("transcripts", "", "directory holding <run_id>/transcript.jsonl; adds the transcript-usage report")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: analyze [-transcripts runs-dir] results.csv")
		os.Exit(2)
	}
	runs, err := readRuns(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "analyze:", err)
		os.Exit(1)
	}
	report(os.Stdout, flag.Arg(0), runs)
	if *transcripts != "" {
		usage, err := readUsage(*transcripts, runs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "analyze:", err)
			os.Exit(1)
		}
		reportUsage(os.Stdout, runs, usage)
	}
}

// readRuns parses the CSV and keeps the final attempt per run_id.
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
	for _, name := range []string{"run_id", "task", "arm", "attempt", "timed_out", "committed", "bypass", "behavior", "corpus", "upkeep", "suite", "hook_blocks", "turns", "wall_s", "output_tokens"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("%s: no %q column", path, name)
		}
	}
	final := map[string]run{}
	for _, row := range rows[1:] {
		get := func(name string) string {
			if i := col[name]; i < len(row) {
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
			TimedOut: get("timed_out") == "true", Committed: get("committed") == "true", Bypass: get("bypass") == "true",
			Behavior: get("behavior"), Corpus: get("corpus"), Upkeep: get("upkeep"), Suite: get("suite"),
			Blocks: int(num("hook_blocks")), Turns: num("turns"), WallMin: num("wall_s") / 60, OutTokens: num("output_tokens"),
		}
		if prev, ok := final[r.ID]; !ok || r.Attempt >= prev.Attempt {
			final[r.ID] = r
		}
	}
	var runs []run
	for _, r := range final {
		runs = append(runs, r)
	}
	sort.Slice(runs, func(a, b int) bool { return runs[a].ID < runs[b].ID })
	return runs, nil
}

// stratum is one task's 2x2 table: successes and runs in arm 3 and arm 2.
type stratum struct {
	Task           string
	X3, N3, X2, N2 int
}

func (s stratum) n() int { return s.N3 + s.N2 }
func (s stratum) m() int { return s.X3 + s.X2 }

// cmh returns the one-sided CMH z (arm 3 above arm 2) without and with the
// continuity correction, and their p values.
func cmh(strata []stratum) (z, p, zCC, pCC float64) {
	var dev, variance float64
	for _, s := range strata {
		n, m := float64(s.n()), float64(s.m())
		if s.n() < 2 {
			continue
		}
		dev += float64(s.X3) - float64(s.N3)*m/n
		variance += float64(s.N3) * float64(s.N2) * m * (n - m) / (n * n * (n - 1))
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
// the null with every stratum's margins fixed, that arm 3 has at least as many
// successes as observed. It convolves the per-task hypergeometric laws.
func exact(strata []stratum) float64 {
	dist := []float64{1}
	observed := 0
	for _, s := range strata {
		observed += s.X3
		lo, hi := max(0, s.m()-s.N2), min(s.N3, s.m())
		next := make([]float64, len(dist)+hi)
		for k := lo; k <= hi; k++ {
			pk := choose(s.N3, k) * choose(s.N2, s.m()-k) / choose(s.n(), s.m())
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

func strata(runs []run) []stratum {
	by := map[string]*stratum{}
	for _, r := range runs {
		if r.Arm != 2 && r.Arm != 3 {
			continue
		}
		s := by[r.Task]
		if s == nil {
			s = &stratum{Task: r.Task}
			by[r.Task] = s
		}
		ok := 0
		if r.Upkeep == "pass" {
			ok = 1
		}
		if r.Arm == 3 {
			s.N3++
			s.X3 += ok
		} else {
			s.N2++
			s.X2 += ok
		}
	}
	var out []stratum
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Task < out[b].Task })
	return out
}

func sums(strata []stratum) (x3, n3, x2, n2 int) {
	for _, s := range strata {
		x3, n3, x2, n2 = x3+s.X3, n3+s.N3, x2+s.X2, n2+s.N2
	}
	return
}

// diffPoints is the pooled arm 3 minus arm 2 upkeep rate, in percentage points.
func diffPoints(strata []stratum) float64 {
	x3, n3, x2, n2 := sums(strata)
	if n3 == 0 || n2 == 0 {
		return 0
	}
	return 100 * (float64(x3)/float64(n3) - float64(x2)/float64(n2))
}

// ceiling returns the tasks at 100% upkeep in both arms 2 and 3.
func ceiling(strata []stratum) (ceil, rest []stratum) {
	for _, s := range strata {
		if s.N3 > 0 && s.N2 > 0 && s.X3 == s.N3 && s.X2 == s.N2 {
			ceil = append(ceil, s)
		} else {
			rest = append(rest, s)
		}
	}
	return
}

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

func report(w io.Writer, path string, runs []run) {
	fmt.Fprintf(w, "# %s: %d runs (final attempt per run_id)\n\n", path, len(runs))

	fmt.Fprintln(w, "## Per task and arm (passes / runs)")
	fmt.Fprintf(w, "%-5s %-3s %-9s %-9s %-9s\n", "task", "arm", "upkeep", "behavior", "corpus")
	for _, task := range tasksOf(runs) {
		for arm := 1; arm <= 3; arm++ {
			var n, up, beh, cor int
			for _, r := range runs {
				if r.Task != task || r.Arm != arm {
					continue
				}
				n++
				if r.Upkeep == "pass" {
					up++
				}
				if r.Behavior == "pass" {
					beh++
				}
				if r.Corpus == "pass" {
					cor++
				}
			}
			if n == 0 {
				continue
			}
			if arm == 1 {
				fmt.Fprintf(w, "%-5s %-3d %-9s %-9s %-9s\n", task, arm, "n/a", fmt.Sprintf("%d/%d", beh, n), "n/a")
				continue
			}
			fmt.Fprintf(w, "%-5s %-3d %-9s %-9s %-9s\n", task, arm, fmt.Sprintf("%d/%d", up, n), fmt.Sprintf("%d/%d", beh, n), fmt.Sprintf("%d/%d", cor, n))
		}
	}

	st := strata(runs)
	x3, n3, x2, n2 := sums(st)
	fmt.Fprintln(w, "\n## Primary: arm 3 against arm 2, upkeep pass, stratified by task")
	if n3 == 0 || n2 == 0 {
		fmt.Fprintln(w, "no arm 2 or arm 3 runs")
		return
	}
	z, p, zCC, pCC := cmh(st)
	pExact := exact(st)
	diff := diffPoints(st)
	fmt.Fprintf(w, "arm 3: %d/%d (%.1f%%)   arm 2: %d/%d (%.1f%%)   pooled difference: %+.1f points\n",
		x3, n3, 100*float64(x3)/float64(n3), x2, n2, 100*float64(x2)/float64(n2), diff)
	fmt.Fprintf(w, "one-sided CMH:                    z = %+.3f  p = %.4f\n", z, p)
	fmt.Fprintf(w, "one-sided CMH, continuity-corr.:  z = %+.3f  p = %.4f\n", zCC, pCC)
	fmt.Fprintf(w, "one-sided exact (stratified):                p = %.4f\n", pExact)

	ceil, rest := ceiling(st)
	var names []string
	for _, s := range ceil {
		names = append(names, s.Task)
	}
	fmt.Fprintf(w, "\nceiling tasks (100%% upkeep in arms 2 and 3): %s\n", orNone(names))
	if len(ceil) > 0 && len(rest) > 0 {
		rx3, rn3, rx2, rn2 := sums(rest)
		rz, rp, _, _ := cmh(rest)
		fmt.Fprintf(w, "without them: arm 3 %d/%d, arm 2 %d/%d, pooled difference %+.1f points, CMH z = %+.3f p = %.4f, exact p = %.4f\n",
			rx3, rn3, rx2, rn2, diffPoints(rest), rz, rp, exact(rest))
	}

	verdict := func(p float64) bool { return p < lookAlpha && diff >= lookDiff }
	fmt.Fprintf(w, "\nlook 1 (p < %.3f and difference >= %.0f points): ", lookAlpha, lookDiff)
	if verdict(p) {
		fmt.Fprintln(w, "arm 3 beats arm 2")
	} else {
		fmt.Fprintln(w, "not detected at this N")
	}
	fmt.Fprintf(w, "  by the corrected CMH: %t; by the exact p: %t (the registered statistic is the plain CMH)\n", verdict(pCC), verdict(pExact))

	fmt.Fprintln(w, "\n## Per-task difference (arm 3 minus arm 2, points)")
	for _, s := range st {
		fmt.Fprintf(w, "%-5s arm 3 %d/%d  arm 2 %d/%d  %+.0f\n", s.Task, s.X3, s.N3, s.X2, s.N2,
			100*(float64(s.X3)/float64(s.N3)-float64(s.X2)/float64(s.N2)))
	}

	fmt.Fprintln(w, "\n## Secondary, per arm (all tasks)")
	fmt.Fprintf(w, "%-3s %-4s %-9s %-9s %-7s %-9s %-11s %-7s %-9s %-12s %-6s\n",
		"arm", "n", "behavior", "committed", "bypass", "timed_out", "hook_blocks", "turns", "wall_min", "out_tokens", "suite")
	for arm := 1; arm <= 3; arm++ {
		var n, beh, com, byp, to, blocks, suite int
		var turns, wall, out []float64
		for _, r := range runs {
			if r.Arm != arm {
				continue
			}
			n++
			beh += b2i(r.Behavior == "pass")
			com += b2i(r.Committed)
			byp += b2i(r.Bypass)
			to += b2i(r.TimedOut)
			suite += b2i(r.Suite == "pass")
			blocks += r.Blocks
			turns, wall, out = append(turns, r.Turns), append(wall, r.WallMin), append(out, r.OutTokens)
		}
		if n == 0 {
			continue
		}
		fmt.Fprintf(w, "%-3d %-4d %-9s %-9s %-7d %-9d %-11d %-7.0f %-9.1f %-12.0f %-6s\n", arm, n,
			frac(beh, n), frac(com, n), byp, to, blocks, median(turns), median(wall), median(out), frac(suite, n))
	}
	fmt.Fprintln(w, "(bypass and timed_out are counts of runs; turns, wall_min and out_tokens are medians)")
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func frac(a, n int) string { return fmt.Sprintf("%d/%d", a, n) }

func orNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// usage is what one run's transcript shows.
type usage struct {
	Found     bool
	Cinch     map[string]int // subcommand -> invocations
	ReadsDocs bool
	ReadsFlow bool
	EditsDoc  bool
	Findings  map[string]int // "checks failed", "make-check failed", "commit error", ...
}

var (
	cinchCall    = regexp.MustCompile(`(?:^|[\s;&|(])(?:\S*/)?cinch\s+([a-z][a-z-]*|--?help|-h)`)
	cinchSubs    = map[string]bool{"check": true, "index": true, "workflows": true, "workflow": true, "rules": true, "render": true, "impact": true, "ignores": true, "upgrade": true, "init": true, "version": true, "help": true, "--help": true, "-h": true}
	readCommand  = regexp.MustCompile(`\b(cat|head|tail|sed|awk|grep|rg|less|bat|nl|cut|xargs)\b`)
	sedInPlace   = regexp.MustCompile(`\b(sed|perl)\s+(-[a-zA-Z]*i|--in-place)`)
	writeToDocs  = regexp.MustCompile(`>>?\s*['"]?\S*\.docs|\btee\b[^|;&]*\.docs|\b(mv|cp|rm)\b[^|;&]*\.docs`)
	hookStepFail = regexp.MustCompile(`(?m)^\s+(checks|make-check|merge-test|field-log)\s+failed`)
	findingLine  = regexp.MustCompile(`(?m)^(links|rules|index|generated|core|commit|hooks) (error|warning)\b`)
)

// scanTranscript reads one transcript.jsonl.
func scanTranscript(path string) (usage, error) {
	u := usage{Cinch: map[string]int{}, Findings: map[string]int{}}
	file, err := os.Open(path)
	if err != nil {
		return u, err
	}
	defer file.Close()
	u.Found = true
	reader := bufio.NewReaderSize(file, 1<<20)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			u.event(line)
		}
		if err == io.EOF {
			return u, nil
		}
		if err != nil {
			return u, err
		}
	}
}

func (u *usage) event(line []byte) {
	var e struct {
		Type     string         `json:"type"`
		ToolName string         `json:"toolName"`
		Args     map[string]any `json:"args"`
		Result   struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	switch {
	case !strings.Contains(string(line), `"tool_execution_`):
		return
	case json.Unmarshal(line, &e) != nil:
		return
	}
	switch e.Type {
	case "tool_execution_start":
		u.call(e.ToolName, e.Args)
	case "tool_execution_end":
		var text strings.Builder
		for _, c := range e.Result.Content {
			text.WriteString(c.Text)
			text.WriteString("\n")
		}
		for _, m := range hookStepFail.FindAllStringSubmatch(text.String(), -1) {
			u.Findings[m[1]+" failed (cinch hook step)"]++
		}
		for _, m := range findingLine.FindAllStringSubmatch(text.String(), -1) {
			u.Findings[m[1]+" "+m[2]]++
		}
	}
}

func (u *usage) call(tool string, args map[string]any) {
	var all []string
	for _, v := range args {
		if s, ok := v.(string); ok {
			all = append(all, s)
		}
	}
	text := strings.Join(all, " ")
	inDocs := strings.Contains(text, ".docs")
	inFlows := strings.Contains(text, ".docs/workflows")
	switch tool {
	case "bash":
		command, _ := args["command"].(string)
		for _, m := range cinchCall.FindAllStringSubmatch(command, -1) {
			if cinchSubs[m[1]] {
				sub := m[1]
				if sub == "--help" || sub == "-h" {
					sub = "help"
				}
				u.Cinch[sub]++
			}
		}
		if inDocs && writeToDocs.MatchString(command) || inDocs && sedInPlace.MatchString(command) {
			u.EditsDoc = true
		} else if inDocs && readCommand.MatchString(command) {
			u.ReadsDocs = true
			u.ReadsFlow = u.ReadsFlow || inFlows
		}
	case "read", "grep":
		if inDocs {
			u.ReadsDocs = true
			u.ReadsFlow = u.ReadsFlow || inFlows
		}
	case "edit", "write":
		if strings.Contains(text, ".docs/") {
			u.EditsDoc = true
		}
	}
}

func readUsage(dir string, runs []run) (map[string]usage, error) {
	out := map[string]usage{}
	for _, r := range runs {
		u, err := scanTranscript(filepath.Join(dir, r.ID, "transcript.jsonl"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out[r.ID] = u
	}
	return out, nil
}

func reportUsage(w io.Writer, runs []run, usages map[string]usage) {
	fmt.Fprintf(w, "\n## Transcript usage, per arm (runs; %d of %d transcripts found)\n", len(usages), len(runs))
	fmt.Fprintf(w, "%-3s %-4s %-14s %-12s %-14s %-12s %-26s\n", "arm", "n", "any cinch", "read .docs", "read workflow", "edited doc", "timed out, no commit")
	subs := map[int]map[string]int{}
	findings := map[int]map[string]int{}
	findingRuns := map[int]map[string]int{}
	for arm := 1; arm <= 3; arm++ {
		var n, anyCinch, docs, flows, edits, stuck int
		subs[arm], findings[arm], findingRuns[arm] = map[string]int{}, map[string]int{}, map[string]int{}
		for _, r := range runs {
			if r.Arm != arm {
				continue
			}
			n++
			stuck += b2i(r.TimedOut && !r.Committed)
			u, ok := usages[r.ID]
			if !ok {
				continue
			}
			calls := 0
			for sub, c := range u.Cinch {
				calls += c
				subs[arm][sub]++
			}
			anyCinch += b2i(calls > 0)
			docs += b2i(u.ReadsDocs)
			flows += b2i(u.ReadsFlow)
			edits += b2i(u.EditsDoc)
			for name, c := range u.Findings {
				findings[arm][name] += c
				findingRuns[arm][name]++
			}
		}
		if n > 0 {
			fmt.Fprintf(w, "%-3d %-4d %-14s %-12s %-14s %-12s %-26s\n", arm, n, frac(anyCinch, n), frac(docs, n), frac(flows, n), frac(edits, n), frac(stuck, n))
		}
	}
	fmt.Fprintln(w, "\n## cinch subcommands invoked by the agent, runs per subcommand")
	for arm := 1; arm <= 3; arm++ {
		fmt.Fprintf(w, "arm %d: %s\n", arm, counts(subs[arm], nil))
	}
	fmt.Fprintln(w, "\n## Hook and check output seen in tool results (occurrences in N runs)")
	for arm := 1; arm <= 3; arm++ {
		fmt.Fprintf(w, "arm %d: %s\n", arm, counts(findings[arm], findingRuns[arm]))
	}
	fmt.Fprintln(w, `
(read .docs: a read or grep tool call, or a bash read command, that names .docs, whether or not the
path exists; arm 1 has no .docs, so its entries are probes that found nothing. edited doc: an edit
or write tool call on a path under .docs/, or a bash write to .docs. Hook output: "<step> failed"
lines of cinch's hook summary, and "<check> error" findings, counted in tool results.)`)
}

// counts renders name n pairs, with "(in r runs)" when runs is given.
func counts(m, runs map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		part := fmt.Sprintf("%s %d", k, m[k])
		if runs != nil {
			part += fmt.Sprintf(" (in %d runs)", runs[k])
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}
