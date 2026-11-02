package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mustRow(t *testing.T, repo, line string) row {
	t.Helper()
	r, err := parseRow(repo, line)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func rowsFrom(t *testing.T, repo string, lines ...string) []row {
	t.Helper()
	var rows []row
	for _, l := range lines {
		rows = append(rows, mustRow(t, repo, l))
	}
	classify(rows)
	return rows
}

func classes(rows []row) []string {
	var c []string
	for _, r := range rows {
		c = append(c, r.class)
	}
	return c
}

func TestClassify(t *testing.T) {
	rows := rowsFrom(t, "r",
		"2026-10-12T10:00:00+03:00\tpre-commit\tf\t-\tclaude\t1\t30\tgenerated error a.md:1: stale|", // blocked
		"2026-10-12T10:01:00+03:00\tpre-commit\tf\t-\tclaude\t0\t30\t",                               // passed
		"2026-10-12T10:01:00+03:00\tpost-commit\tf\taaa\tclaude\t0\t90\t",                            // clean
		"2026-10-12T10:02:00+03:00\tpre-commit\tf\t-\tclaude\t0\t30\t",                               // rejected-later
		"2026-10-12T10:03:00+03:00\tpost-commit\tg\tbbb\tclaude\t1\t90\trules error x.md:2: bad|",    // red-landed, no pre-commit before: bypass
		"2026-10-12T10:04:00+03:00\tpre-commit\tdev\t-\thuman\t0\t30\t",                              // passed (other branch interleaves)
		"2026-10-12T10:04:00+03:00\tpost-commit\tdev\tccc\thuman\t2\t11\t",                           // superseded by the next row
		"2026-10-12T10:04:00+03:00\tpost-commit\tdev\tccc\thuman\t0\t100\t",                          // clean
	)
	want := []string{classBlocked, classPassed, classClean, classRejectedLate, classRedLanded, classPassed, classSuperseded, classClean}
	if got := classes(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("classes = %v, want %v", got, want)
	}
	wantBypass := []bool{false, false, false, false, true, false, false, false}
	for i, r := range rows {
		if r.event == "post-commit" && r.bypass != wantBypass[i] {
			t.Errorf("row %d bypass = %v, want %v", i, r.bypass, wantBypass[i])
		}
	}
}

func TestFailedPreCommitFollowedByPostIsNotABlock(t *testing.T) {
	rows := rowsFrom(t, "r",
		"2026-10-12T10:00:00+03:00\tpre-commit\tf\t-\tclaude\t1\t30\tcore block x: y|",
		"2026-10-12T10:00:01+03:00\tpost-commit\tf\taaa\tclaude\t1\t90\tcore block x: y|",
	)
	if rows[0].class != classFailedLanded || rows[1].class != classRedLanded {
		t.Fatalf("classes = %v", classes(rows))
	}
}

func TestParseRowKeepsEmptyFindings(t *testing.T) {
	r := mustRow(t, "r", "2026-10-06T21:32:35+03:00\tpre-commit\tmain\t-\thuman\t0\t67\t")
	if r.findings != "" || r.ms != 67 || r.actor != "human" {
		t.Fatalf("row = %+v", r)
	}
	if _, err := parseRow("r", "2026-10-06T21:32:35+03:00\tbogus\tmain\t-\thuman\t0\t67\t"); err == nil {
		t.Fatal("unknown event accepted")
	}
}

func TestDistinctFindingsIgnoreLineNumbers(t *testing.T) {
	rows := rowsFrom(t, "r",
		"2026-10-12T10:00:00+03:00\tpre-commit\tf\t-\tc\t1\t3\trules error t.go:10: // cinch:rule X-1 does not resolve|rules error t.go:20: // cinch:rule X-1 does not resolve|rules error u.go:1: // cinch:rule X-1 does not resolve|",
		"2026-10-12T10:01:00+03:00\tpre-commit\tf\t-\tc\t1\t3\trules error t.go:11: // cinch:rule X-1 does not resolve|",
	)
	got := distinctFindings(rows)
	if len(got) != 2 {
		t.Fatalf("distinct = %+v, want 2 (t.go and u.go)", got)
	}
	if got[0].file != "t.go" || got[0].rows != 2 || got[1].file != "u.go" || got[1].rows != 1 {
		t.Fatalf("distinct = %+v", got)
	}
}

func TestRank(t *testing.T) {
	v := []int{50, 10, 20, 30, 40, 60, 70, 80, 90, 100}
	if median(v) != 50 || p90(v) != 90 {
		t.Fatalf("median %d p90 %d", median(v), p90(v))
	}
	if median(nil) != 0 {
		t.Fatal("empty median")
	}
}

func TestSummaryExcludesEarlyDeltaDocsMSAndSupersededRows(t *testing.T) {
	rows := rowsFrom(t, "project_deltadocs",
		"2026-10-06T21:32:03+03:00\tpre-commit\tf\t-\thuman\t0\t873\t",
		"2026-10-06T21:32:04+03:00\tpost-commit\tf\taaa\thuman\t0\t947\t",
		"2026-10-09T16:13:26+03:00\tpre-commit\tdev\t-\thuman\t0\t33\t",
		"2026-10-09T16:13:26+03:00\tpost-commit\tdev\tbbb\thuman\t2\t12\t",
		"2026-10-09T16:13:26+03:00\tpost-commit\tdev\tbbb\thuman\t0\t105\t",
	)
	s := summarize("project_deltadocs", rows)
	if s.commits != 2 || s.redLanded != 0 || s.superseded != 1 || s.supersededNonZero != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if !reflect.DeepEqual(s.preMS, []int{33}) || !reflect.DeepEqual(s.postMS, []int{105}) {
		t.Fatalf("ms = %v / %v", s.preMS, s.postMS)
	}
	if !strings.Contains(renderSummary([]stats{s}), "| project_deltadocs | 5 | 2 | 0 | 0 | 0 | 2 / 0 / 0 |") {
		t.Fatalf("summary:\n%s", renderSummary([]stats{s}))
	}
}

func TestHookLogCSVHasNoCommitSubjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-log.csv")
	rows := rowsFrom(t, "r", "2026-10-06T21:32:35+03:00\tpre-commit\tmain\t-\thuman\t0\t67\t")
	if err := writeHookLog(path, rows); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	head := strings.SplitN(string(b), "\n", 2)[0]
	if head != "repo,time,event,branch,head,actor,exit,ms,class,bypass_candidate,findings" {
		t.Fatalf("header = %s", head)
	}
}

func TestDelta(t *testing.T) {
	old := ruleSet{"A-1": {ID: "A-1", Text: "1. **A-1** one"}, "A-2": {ID: "A-2", Text: "2. **A-2** two"}, "A-3": {ID: "A-3", Text: "3. **A-3** gone"}}
	cur := ruleSet{"A-1": {ID: "A-1", Text: "4.  **A-1**   one"}, "A-2": {ID: "A-2", Text: "2. **A-2** two, tightened"}, "A-4": {ID: "A-4", Text: "x"}}
	added, changed, removed := delta(old, cur)
	if !reflect.DeepEqual(added, []string{"A-4"}) || !reflect.DeepEqual(changed, []string{"A-2"}) || !reflect.DeepEqual(removed, []string{"A-3"}) {
		t.Fatalf("delta = %v %v %v (renumbering A-1 must not count)", added, changed, removed)
	}
}

func TestIsProductExcludesAgentDirs(t *testing.T) {
	p := []string{"backend/", "frontend/"}
	for path, want := range map[string]bool{
		"backend/x.go":                true,
		"frontend/src/a.ts":           true,
		"frontend/.agent/note.md":     false,
		".agent/domain/x.md":          false,
		"docs/x.md":                   false,
		"backend/tests/a_test.go":     true,
		"scripts/post-commit-bump.sh": false,
	} {
		if got := isProduct(path, p); got != want {
			t.Errorf("isProduct(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestHeadlineCountsFeatureAndFixOnly(t *testing.T) {
	rows := []merge{
		{kind: "feature", product: true, deltaOK: true, added: []string{"A-1"}, parentRules: 3, date: "2026-10-10T00:00:00+03:00"},
		{kind: "fix", product: true, deltaOK: true, parentRules: 3, date: "2026-10-11T00:00:00+03:00"},
		{kind: "fix", product: false, deltaOK: true, added: []string{"A-2"}, parentRules: 3, date: "2026-10-12T00:00:00+03:00"},  // no product code: out
		{kind: "chore", product: true, deltaOK: true, added: []string{"A-3"}, parentRules: 3, date: "2026-10-13T00:00:00+03:00"}, // wrong type: out
	}
	out := headline(rows, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(out, "rule era (the first parent already had rules): 4 merges; 1 of 2 feature/fix merges touching product code added or changed a rule (50%)") {
		t.Fatalf("headline:\n%s", out)
	}
}

// TestWalkOnARealRepo replays a tiny repository with a stub rule reader: the
// rules are the lines of rules.txt ("ID text").
func TestWalkOnARealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	sh := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		p := filepath.Join(repo, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sh(repo, "init", "-q", "-b", "dev")
	write("rules.txt", "A-1 one\n")
	sh(repo, "add", "-A")
	sh(repo, "commit", "-q", "-m", "base")
	write("rules.txt", "A-1 one\nA-2 two\n")
	write("lib/x.go", "package x\n")
	sh(repo, "add", "-A")
	sh(repo, "commit", "-q", "-m", "merge: feature/two")
	write("rules.txt", "A-1 one\nA-2 two\n")
	write("lib/y.go", "package y\n")
	sh(repo, "add", "-A")
	sh(repo, "commit", "-q", "-m", "merge: fix/no-rule")
	write("docs/z.md", "z\n")
	sh(repo, "add", "-A")
	sh(repo, "commit", "-q", "-m", "merge: dev")

	merges, err := listMerges(repo, "dev", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 2 {
		t.Fatalf("merges = %+v, want 2 (merge: dev is a sync merge)", merges)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	sh(repo, "worktree", "add", "--detach", wt, merges[0].hash)
	stub := func(dir string) (ruleSet, error) {
		b, err := os.ReadFile(filepath.Join(dir, "rules.txt"))
		if err != nil {
			return nil, err
		}
		rs := ruleSet{}
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			id, text, _ := strings.Cut(l, " ")
			rs[id] = rule{ID: id, Text: text}
		}
		return rs, nil
	}
	rows, err := walk(repo, wt, merges, []string{"lib/"}, stub)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].product || !reflect.DeepEqual(rows[0].added, []string{"A-2"}) || rows[0].rules != 2 {
		t.Fatalf("first row = %+v", rows[0])
	}
	if !rows[1].product || len(rows[1].added)+len(rows[1].changed) != 0 || !rows[1].prevOK {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[0].kind != "feature" || rows[1].kind != "fix" {
		t.Fatalf("kinds = %s %s", rows[0].kind, rows[1].kind)
	}
}
