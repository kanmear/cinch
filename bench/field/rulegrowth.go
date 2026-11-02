package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// rule is the part of one `cinch rules --json` entry the replay compares.
type rule struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	HasIgnore bool   `json:"hasIgnore"`
	Markers   []struct {
		File string `json:"file"`
		Line int    `json:"line"`
	} `json:"markers"`
}

type ruleSet map[string]rule

// rulesFunc returns the rule inventory of the checkout in dir.
type rulesFunc func(dir string) (ruleSet, error)

var numbering = regexp.MustCompile(`^\d+\.\s+`)

// normText strips list numbering and whitespace runs, so renumbering a list is
// not a rule change.
func normText(s string) string {
	return strings.Join(strings.Fields(numbering.ReplaceAllString(strings.TrimSpace(s), "")), " ")
}

// delta lists the IDs added, changed (same ID, different text) and removed
// going from old to cur.
func delta(old, cur ruleSet) (added, changed, removed []string) {
	for id, r := range cur {
		o, ok := old[id]
		switch {
		case !ok:
			added = append(added, id)
		case normText(o.Text) != normText(r.Text):
			changed = append(changed, id)
		}
	}
	for id := range old {
		if _, ok := cur[id]; !ok {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(changed)
	sort.Strings(removed)
	return
}

func counts(rs ruleSet) (total, marked, ignored int) {
	for _, r := range rs {
		total++
		if len(r.Markers) > 0 {
			marked++
		}
		if r.HasIgnore {
			ignored++
		}
	}
	return
}

// cinchRules runs `cinch rules --json` in dir.
func cinchRules(dir string) (ruleSet, error) {
	cmd := exec.Command("cinch", "rules", "--json")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var out struct {
		Rules []rule `json:"rules"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		return nil, fmt.Errorf("%s", oneLine(msg))
	}
	rs := ruleSet{}
	for _, r := range out.Rules {
		rs[r.ID] = r
	}
	return rs, nil
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// merge is one `merge: <name>` commit and what it did to the rules.
type merge struct {
	hash, date, subject, name, kind string
	product                         bool

	rules, marked, ignored         int
	added, changed, removed        []string // against the first parent
	prevAdded, prevChanged, prevRm []string // against the previous row
	parentRules                    int
	deltaOK, prevOK                bool
	err                            string
}

type commit struct{ hash, date, subject string }

var mergeSubject = regexp.MustCompile(`^merge: (\S.*)$`)

// listMerges returns, oldest first, every commit reachable from branch whose
// subject is `merge: <name>`. Sync merges (`merge: dev`, `merge: main`) are not
// feature merges and are left out. Squash merges onto a local branch can sit
// behind such a sync merge, off the first-parent line, so this walks all
// ancestors rather than only the first-parent chain.
func listMerges(repo, branch string, since time.Time) ([]commit, error) {
	out, err := git(repo, "log", "--reverse", "--topo-order", "--format=%H%x09%cI%x09%s", branch)
	if err != nil {
		return nil, err
	}
	var res []commit
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) < 3 {
			continue
		}
		m := mergeSubject.FindStringSubmatch(f[2])
		if m == nil || m[1] == "dev" || m[1] == "main" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, f[1]); err == nil && t.Before(since) {
			continue
		}
		res = append(res, commit{f[0], f[1], f[2]})
	}
	return res, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, oneLine(stderr.String()))
	}
	return string(out), nil
}

// isProduct reports whether path is product code: under one of prefixes and
// not inside an .agent/ directory.
func isProduct(path string, prefixes []string) bool {
	if strings.HasPrefix(path, ".agent/") || strings.Contains(path, "/.agent/") {
		return false
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func kindOf(name string) string {
	if i := strings.Index(name, "/"); i > 0 {
		return name[:i]
	}
	return name
}

// walk replays the merges. run is called with the checkout directory after
// each `git checkout`; wt is a detached worktree of repo.
func walk(repo, wt string, merges []commit, prefixes []string, run rulesFunc) ([]merge, error) {
	type state struct {
		rs  ruleSet
		err error
	}
	cache := map[string]state{}
	at := func(rev string) state {
		if s, ok := cache[rev]; ok {
			return s
		}
		var s state
		if _, err := git(wt, "checkout", "-q", "-f", "--detach", rev); err != nil {
			s.err = err
		} else {
			s.rs, s.err = run(wt)
		}
		cache[rev] = s
		return s
	}

	var rows []merge
	var prev *state
	for _, c := range merges {
		m := merge{hash: c.hash, date: c.date, subject: c.subject}
		m.name = mergeSubject.FindStringSubmatch(c.subject)[1]
		m.kind = kindOf(m.name)

		files, err := git(repo, "diff", "--name-only", "-z", c.hash+"^1", c.hash)
		if err != nil {
			m.err = err.Error()
		}
		for _, f := range strings.Split(files, "\x00") {
			if f != "" && isProduct(f, prefixes) {
				m.product = true
				break
			}
		}

		cur := at(c.hash)
		par := at(c.hash + "^1")
		if cur.err != nil {
			m.err = "merge: " + cur.err.Error()
		} else {
			m.rules, m.marked, m.ignored = counts(cur.rs)
		}
		if par.err == nil {
			m.parentRules = len(par.rs)
		}
		if cur.err == nil && par.err == nil {
			m.added, m.changed, m.removed = delta(par.rs, cur.rs)
			m.deltaOK = true
		} else if par.err != nil && m.err == "" {
			m.err = "parent: " + par.err.Error()
		}
		if cur.err == nil && prev != nil && prev.err == nil {
			m.prevAdded, m.prevChanged, m.prevRm = delta(prev.rs, cur.rs)
			m.prevOK = true
		}
		s := cur
		prev = &s
		rows = append(rows, m)
	}
	return rows, nil
}

// eligible: a feature/ or fix/ merge that touched product code and whose rule
// delta is known.
func eligible(m merge) bool {
	return (m.kind == "feature" || m.kind == "fix") && m.product && m.deltaOK
}

func grew(m merge) bool { return len(m.added)+len(m.changed) > 0 }

// window selects which merges a headline covers.
type window struct {
	name string
	in   func(merge) bool
}

func windows(fieldStart time.Time) []window {
	return []window{
		{"all merges", func(merge) bool { return true }},
		{"rule era (the first parent already had rules)", func(m merge) bool { return m.parentRules > 0 }},
		{"field-log window (from " + fieldStart.Format("2006-01-02") + ")", func(m merge) bool {
			t, err := time.Parse(time.RFC3339, m.date)
			return err == nil && !t.Before(fieldStart)
		}},
	}
}

// headline renders plan 20's number for each window.
func headline(rows []merge, fieldStart time.Time) string {
	var b strings.Builder
	for _, w := range windows(fieldStart) {
		var merges, eligibleN, grewN, addedRules, changedRules int
		kinds := map[string][3]int{} // merges, touching product, grew
		for _, m := range rows {
			if !w.in(m) {
				continue
			}
			merges++
			k := kinds[m.kind]
			k[0]++
			if m.product {
				k[1]++
			}
			if m.deltaOK && grew(m) {
				k[2]++
			}
			kinds[m.kind] = k
			addedRules += len(m.added)
			changedRules += len(m.changed)
			if eligible(m) {
				eligibleN++
				if grew(m) {
					grewN++
				}
			}
		}
		share := "n/a"
		if eligibleN > 0 {
			share = fmt.Sprintf("%.0f%%", 100*float64(grewN)/float64(eligibleN))
		}
		fmt.Fprintf(&b, "%s: %d merges; %d of %d feature/fix merges touching product code added or changed a rule (%s); %d rules added, %d changed\n",
			w.name, merges, grewN, eligibleN, share, addedRules, changedRules)
		var names []string
		for k := range kinds {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			v := kinds[k]
			fmt.Fprintf(&b, "    %-9s %2d merges, %2d touch product code, %2d added or changed a rule\n", k, v[0], v[1], v[2])
		}
	}
	return b.String()
}

func writeRuleGrowth(path string, rows []merge) error {
	recs := [][]string{{"commit", "date", "subject", "type", "product_code", "rules", "marked", "ignored",
		"added", "changed", "removed", "added_ids", "changed_ids", "removed_ids",
		"prev_row_added", "prev_row_changed", "prev_row_removed", "error"}}
	n := func(ok bool, v []string) string {
		if !ok {
			return ""
		}
		return strconv.Itoa(len(v))
	}
	for _, m := range rows {
		rules, marked, ignored := "", "", ""
		if !strings.HasPrefix(m.err, "merge:") {
			rules, marked, ignored = strconv.Itoa(m.rules), strconv.Itoa(m.marked), strconv.Itoa(m.ignored)
		}
		recs = append(recs, []string{m.hash[:7], m.date, m.subject, m.kind, strconv.FormatBool(m.product),
			rules, marked, ignored,
			n(m.deltaOK, m.added), n(m.deltaOK, m.changed), n(m.deltaOK, m.removed),
			strings.Join(m.added, "|"), strings.Join(m.changed, "|"), strings.Join(m.removed, "|"),
			n(m.prevOK, m.prevAdded), n(m.prevOK, m.prevChanged), n(m.prevOK, m.prevRm), m.err})
	}
	return writeCSV(path, recs)
}

func rulegrowthCmd(args []string) error {
	fs := flag.NewFlagSet("rulegrowth", flag.ExitOnError)
	dir := fs.String("o", filepath.Join("bench", "field"), "output directory")
	name := fs.String("name", "", "repo name for the output file (default: basename of REPO)")
	sinceFlag := fs.String("since", "", "only merges committed on or after this date (YYYY-MM-DD)")
	product := fs.String("product", "", "comma-separated product-code prefixes (default by repo name: budgetron lib/, project_deltadocs backend/ and frontend/)")
	tmp := fs.String("tmp", "", "directory for the temporary worktree (default: system temp)")
	fieldStart := fs.String("field-start", "2026-10-06", "start of the field-log window, YYYY-MM-DD")
	fs.Parse(args)
	if fs.NArg() != 2 {
		usage()
	}
	repo, branch := fs.Arg(0), fs.Arg(1)
	if *name == "" {
		abs, _ := filepath.Abs(repo)
		*name = filepath.Base(abs)
	}
	if *product == "" {
		switch *name {
		case "budgetron":
			*product = "lib/"
		case "project_deltadocs":
			*product = "backend/,frontend/"
		default:
			return fmt.Errorf("no default product prefixes for %q; pass -product", *name)
		}
	}
	var since time.Time
	if *sinceFlag != "" {
		t, err := time.Parse("2006-01-02", *sinceFlag)
		if err != nil {
			return err
		}
		since = t
	}
	start, err := time.Parse("2006-01-02", *fieldStart)
	if err != nil {
		return err
	}

	merges, err := listMerges(repo, branch, since)
	if err != nil {
		return err
	}
	if len(merges) == 0 {
		return fmt.Errorf("no `merge: <name>` commits reachable from %s", branch)
	}

	wt, err := os.MkdirTemp(*tmp, "rulegrowth-")
	if err != nil {
		return err
	}
	wt = filepath.Join(wt, "wt")
	if _, err := git(repo, "worktree", "add", "--detach", wt, merges[0].hash); err != nil {
		return err
	}
	defer func() {
		git(repo, "worktree", "remove", "--force", wt)
		os.RemoveAll(filepath.Dir(wt))
		git(repo, "worktree", "prune")
	}()

	rows, err := walk(repo, wt, merges, strings.Split(*product, ","), cinchRules)
	if err != nil {
		return err
	}
	out := filepath.Join(*dir, "rule-growth-"+*name+".csv")
	if err := writeRuleGrowth(out, rows); err != nil {
		return err
	}
	fmt.Printf("%s %s: %d merges -> %s\n%s", *name, branch, len(rows), out, headline(rows, start))
	return nil
}
