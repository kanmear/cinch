package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// row is one line of a hook log (plan 19): time, event, branch, head, actor,
// exit, ms, findings.
type row struct {
	repo     string
	time     time.Time
	rawTime  string
	event    string
	branch   string
	head     string
	actor    string
	exit     int
	ms       int
	findings string

	class  string // see classify
	bypass bool   // post-commit with no pre-commit before it on that branch
}

// Classes. A pre-commit row is passed, blocked, rejected-later or
// failed-landed; a post-commit row is clean, red-landed or superseded.
const (
	classPassed       = "passed"
	classBlocked      = "blocked"
	classRejectedLate = "rejected-later"
	classFailedLanded = "failed-landed"
	classClean        = "clean"
	classRedLanded    = "red-landed"
	classSuperseded   = "superseded"
)

// costCutoff: DeltaDocs rows before this carry ~800 ms from a tracked binary
// (plan 19, fixed in cinch 0.9.1), so they are left out of the ms figures.
var costCutoff = time.Date(2026, 10, 7, 0, 0, 0, 0, time.FixedZone("", 3*3600))

// repoName names the repo a log belongs to: the directory holding its .git.
func repoName(logPath string) string {
	abs, err := filepath.Abs(logPath)
	if err != nil {
		abs = logPath
	}
	return filepath.Base(filepath.Dir(filepath.Dir(abs)))
}

func readLog(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	repo := repoName(path)
	var rows []row
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" {
			continue
		}
		r, err := parseRow(repo, line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, n, err)
		}
		rows = append(rows, r)
	}
	return rows, sc.Err()
}

func parseRow(repo, line string) (row, error) {
	f := strings.SplitN(line, "\t", 8)
	if len(f) < 7 {
		return row{}, fmt.Errorf("want 8 tab-separated columns, got %d", len(f))
	}
	t, err := time.Parse(time.RFC3339, f[0])
	if err != nil {
		return row{}, err
	}
	exit, err := strconv.Atoi(f[5])
	if err != nil {
		return row{}, fmt.Errorf("exit: %v", err)
	}
	ms, err := strconv.Atoi(f[6])
	if err != nil {
		return row{}, fmt.Errorf("ms: %v", err)
	}
	r := row{repo: repo, time: t, rawTime: f[0], event: f[1], branch: f[2], head: f[3], actor: f[4], exit: exit, ms: ms}
	if len(f) == 8 {
		r.findings = f[7]
	}
	if r.event != "pre-commit" && r.event != "post-commit" {
		return row{}, fmt.Errorf("unknown event %q", r.event)
	}
	return r, nil
}

// classify sets class and bypass on every row of one repo's log, in file order.
//
//   - A post-commit row is superseded when a later post-commit row has the same
//     head: a version-bump amend fires post-commit twice, and the last row is
//     the verdict on what landed. Otherwise it is clean or, with a non-zero
//     exit, red-landed.
//   - A pre-commit row looks at the next row on the same branch. Followed by a
//     post-commit, it passed (exit 0) or failed-landed (non-zero: the commit
//     went through anyway). Otherwise no commit followed: a non-zero exit is a
//     block, and exit 0 is rejected-later (an empty commit, or a project hook
//     or commit-msg refused it).
//   - A post-commit row is a bypass candidate when the nearest earlier row on
//     its branch, skipping post-commit rows for the same head, is not a
//     pre-commit: git skipped the hook (--no-verify) or it never ran.
func classify(rows []row) {
	lastPost := map[string]int{}
	for i, r := range rows {
		if r.event == "post-commit" {
			lastPost[r.head] = i
		}
	}
	for i := range rows {
		r := &rows[i]
		if r.event == "post-commit" {
			switch {
			case lastPost[r.head] != i:
				r.class = classSuperseded
			case r.exit == 0:
				r.class = classClean
			default:
				r.class = classRedLanded
			}
			r.bypass = true
			for j := i - 1; j >= 0; j-- {
				p := rows[j]
				if p.branch != r.branch || (p.event == "post-commit" && p.head == r.head) {
					continue
				}
				r.bypass = p.event != "pre-commit"
				break
			}
			continue
		}
		followedByPost := false
		for j := i + 1; j < len(rows); j++ {
			if rows[j].branch == r.branch {
				followedByPost = rows[j].event == "post-commit"
				break
			}
		}
		switch {
		case followedByPost && r.exit == 0:
			r.class = classPassed
		case followedByPost:
			r.class = classFailedLanded
		case r.exit != 0:
			r.class = classBlocked
		default:
			r.class = classRejectedLate
		}
	}
}

// finding is one distinct finding: same check, file and message (line numbers
// move, so they are not part of the identity).
type finding struct {
	repo, check, level, file, message string
	rows                              int
	first, last                       string
}

var findingRE = regexp.MustCompile(`^([a-z]+) (error|block) ([^:]+?)(?::\d+)?: (.*)$`)

func parseFindings(s string) (out [][4]string) {
	for _, part := range strings.Split(s, "|") {
		if part == "" {
			continue
		}
		if m := findingRE.FindStringSubmatch(part); m != nil {
			out = append(out, [4]string{m[1], m[2], m[3], m[4]})
		} else {
			out = append(out, [4]string{"?", "?", "", part})
		}
	}
	return out
}

// distinctFindings counts each (repo, check, file, message) once, with the
// number of log rows it appeared in.
func distinctFindings(rows []row) []finding {
	idx := map[string]*finding{}
	var order []string
	for _, r := range rows {
		seen := map[string]bool{}
		for _, p := range parseFindings(r.findings) {
			key := strings.Join(append([]string{r.repo}, p[:]...), "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			f := idx[key]
			if f == nil {
				f = &finding{repo: r.repo, check: p[0], level: p[1], file: p[2], message: p[3], first: r.rawTime}
				idx[key] = f
				order = append(order, key)
			}
			f.rows++
			f.last = r.rawTime
		}
	}
	out := make([]finding, 0, len(order))
	for _, k := range order {
		out = append(out, *idx[k])
	}
	return out
}

// stats is one repo's summary line.
type stats struct {
	repo                          string
	rows, commits                 int
	blocks, rejected, failedLand  int
	redLanded, superseded         int
	supersededNonZero, bypassCand int
	actors                        map[string]int // commits by actor
	preMS, postMS                 []int
}

func summarize(repo string, rows []row) stats {
	s := stats{repo: repo, rows: len(rows), actors: map[string]int{}}
	for _, r := range rows {
		counted := !(repo == "project_deltadocs" && r.time.Before(costCutoff)) && r.class != classSuperseded
		switch r.class {
		case classBlocked:
			s.blocks++
		case classRejectedLate:
			s.rejected++
		case classFailedLanded:
			s.failedLand++
		case classRedLanded:
			s.redLanded++
		case classSuperseded:
			s.superseded++
			if r.exit != 0 {
				s.supersededNonZero++
			}
		}
		if r.event == "post-commit" && r.class != classSuperseded {
			s.commits++
			s.actors[r.actor]++
			if r.bypass {
				s.bypassCand++
			}
		}
		if counted {
			if r.event == "pre-commit" {
				s.preMS = append(s.preMS, r.ms)
			} else {
				s.postMS = append(s.postMS, r.ms)
			}
		}
	}
	return s
}

// median and p90 use the nearest-rank method, so both are observed values.
func median(v []int) int { return rank(v, 0.5) }
func p90(v []int) int    { return rank(v, 0.9) }

func rank(v []int, q float64) int {
	if len(v) == 0 {
		return 0
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	k := int(q*float64(len(s)) + 0.9999999)
	if k < 1 {
		k = 1
	}
	return s[k-1]
}

func hooklogCmd(args []string) error {
	fs := flag.NewFlagSet("hooklog", flag.ExitOnError)
	dir := fs.String("o", filepath.Join("bench", "field"), "output directory")
	fs.Parse(args)
	if fs.NArg() == 0 {
		usage()
	}

	var all []row
	var table []stats
	for _, path := range fs.Args() {
		rows, err := readLog(path)
		if err != nil {
			return err
		}
		classify(rows)
		table = append(table, summarize(repoName(path), rows))
		all = append(all, rows...)
	}
	if err := writeHookLog(filepath.Join(*dir, "hook-log.csv"), all); err != nil {
		return err
	}
	if err := writeFindings(filepath.Join(*dir, "findings.csv"), distinctFindings(all)); err != nil {
		return err
	}
	fmt.Print(renderSummary(table))
	return nil
}

func writeHookLog(path string, rows []row) error {
	var recs [][]string
	recs = append(recs, []string{"repo", "time", "event", "branch", "head", "actor", "exit", "ms", "class", "bypass_candidate", "findings"})
	for _, r := range rows {
		bypass := ""
		if r.event == "post-commit" && r.bypass {
			bypass = "yes"
		}
		recs = append(recs, []string{r.repo, r.rawTime, r.event, r.branch, r.head, r.actor,
			strconv.Itoa(r.exit), strconv.Itoa(r.ms), r.class, bypass, r.findings})
	}
	return writeCSV(path, recs)
}

func writeFindings(path string, fs []finding) error {
	recs := [][]string{{"repo", "check", "level", "file", "message", "rows", "first_seen", "last_seen"}}
	for _, f := range fs {
		recs = append(recs, []string{f.repo, f.check, f.level, f.file, f.message, strconv.Itoa(f.rows), f.first, f.last})
	}
	return writeCSV(path, recs)
}

func writeCSV(path string, recs [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(recs); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func renderSummary(table []stats) string {
	var b strings.Builder
	b.WriteString("| Repo | Rows | Commits | Blocks | Rejected later | Red landed | Commits by human / claude / pi | Pre-commit ms (median, p90) | Post-commit ms (median, p90) |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, s := range table {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d / %d / %d | %d, %d | %d, %d |\n",
			s.repo, s.rows, s.commits, s.blocks, s.rejected, s.redLanded,
			s.actors["human"], s.actors["claude"], s.actors["pi"],
			median(s.preMS), p90(s.preMS), median(s.postMS), p90(s.postMS))
	}
	b.WriteString("\n| Repo | Superseded post-commit rows | of which non-zero exit | Failed pre-commit, commit landed | Post-commit with no pre-commit before it |\n|---|---|---|---|---|\n")
	for _, s := range table {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", s.repo, s.superseded, s.supersededNonZero, s.failedLand, s.bypassCand)
	}
	return b.String()
}
