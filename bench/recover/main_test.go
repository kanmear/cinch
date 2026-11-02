package main

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseQuestions(t *testing.T) {
	good := `{"repo":"budgetron","commit":"abc","questions":[
	  {"id":"CAT-001","doc":"d.md","question":"q","key":"k","notes":"n","predict":"normative","ignored":false},
	  {"id":"CAT-002","doc":"d.md","question":"","key":"","notes":"","predict":"","ignored":true}]}`
	f, err := parseQuestions([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if f.Repo != "budgetron" || f.Commit != "abc" || len(f.Questions) != 2 {
		t.Fatalf("parsed %+v", f)
	}
	if active := f.active(); len(active) != 1 || active[0].ID != "CAT-001" {
		t.Errorf("active = %+v", active)
	}
	bad := map[string]string{
		"unknown field": `{"repo":"r","commit":"c","questions":[{"id":"A","doc":"d","question":"q","key":"k","predict":"normative","extra":1}]}`,
		"duplicate id":  `{"repo":"r","commit":"c","questions":[{"id":"A","doc":"d","question":"q","key":"k","predict":"normative"},{"id":"A","doc":"d","question":"q","key":"k","predict":"normative"}]}`,
		"bad predict":   `{"repo":"r","commit":"c","questions":[{"id":"A","doc":"d","question":"q","key":"k","predict":"maybe"}]}`,
		"no key":        `{"repo":"r","commit":"c","questions":[{"id":"A","doc":"d","question":"q","key":"","predict":"normative"}]}`,
		"no commit":     `{"repo":"r","questions":[{"id":"A","doc":"d","question":"q","key":"k","predict":"normative"}]}`,
		"no questions":  `{"repo":"r","commit":"c","questions":[]}`,
	}
	for name, data := range bad {
		if _, err := parseQuestions([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := loadQuestions(filepath.Join("testdata", "dryrun-questions.json")); err != nil {
		t.Errorf("testdata questions: %v", err)
	}
}

func testQuestions(n int) []question {
	var qs []question
	for i := 0; i < n; i++ {
		qs = append(qs, question{ID: "Q-" + string(rune('A'+i))})
	}
	return qs
}

func ids(jobs []job) []string {
	var out []string
	for _, j := range jobs {
		out = append(out, j.id())
	}
	return out
}

func TestScheduleDeterminism(t *testing.T) {
	qs := testQuestions(6)
	a := schedule(20261101, "pi", qs, 3)
	b := schedule(20261101, "pi", qs, 3)
	if !reflect.DeepEqual(ids(a), ids(b)) {
		t.Fatal("same seed and engine gave different orders")
	}
	if len(a) != 6*2*3 {
		t.Fatalf("%d jobs, want 36", len(a))
	}
	if reflect.DeepEqual(ids(a), ids(schedule(20261101, "fake", qs, 3))) {
		t.Error("two engines share an order")
	}
	if reflect.DeepEqual(ids(a), ids(schedule(1, "pi", qs, 3))) {
		t.Error("two seeds share an order")
	}
	seen := map[string]bool{}
	for i, j := range a {
		if j.Position != i || seen[j.id()] {
			t.Fatalf("bad job %d: %+v", i, j)
		}
		seen[j.id()] = true
	}
	// A subset keeps the positions it had in the full shuffle.
	sub := selectJobs(a, []string{armDocs}, []string{"Q-B", "Q-D"})
	if len(sub) != 2*3 {
		t.Fatalf("subset has %d jobs", len(sub))
	}
	for _, j := range sub {
		if a[j.Position].id() != j.id() {
			t.Errorf("%s moved", j.id())
		}
	}
}

func TestScheduleGolden(t *testing.T) {
	got := ids(schedule(20261101, "pi", testQuestions(2), 1))
	t.Logf("order: %v", got)
	want := []string{"Q-B-docs-r0", "Q-B-code-r0", "Q-A-docs-r0", "Q-A-code-r0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed: %v, want %v", got, want)
	}
}

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDeriveAgents(t *testing.T) {
	pin := readFile(t, "testdata", "pin-AGENTS.md")
	code, err := deriveAgents(pin, armCode, ".docs")
	if err != nil {
		t.Fatal(err)
	}
	if want := readFile(t, "testdata", "want-AGENTS-code.md"); code != want {
		t.Errorf("code arm AGENTS.md:\n%q\nwant\n%q", code, want)
	}
	docs, err := deriveAgents(pin, armDocs, ".docs")
	if err != nil {
		t.Fatal(err)
	}
	if want := readFile(t, "testdata", "want-AGENTS-docs.md"); docs != want {
		t.Errorf("docs arm AGENTS.md:\n%q\nwant\n%q", docs, want)
	}
	if strings.Contains(docs, "`cinch ") || !strings.Contains(docs, ".docs/") {
		t.Errorf("docs arm must keep the pointer and drop the cinch commands:\n%s", docs)
	}
	if _, err := deriveAgents("# x\n", armDocs, ".docs"); err == nil {
		t.Error("docs arm accepted an AGENTS.md without a pointer")
	}
	if _, err := deriveAgents("# x\n\nRead .docs/ first.\n\nAlso cinch check.\n", armCode, ".docs"); err == nil {
		t.Error("code arm accepted leftover cinch mentions")
	}
}

// fixture builds a tiny working repo shaped like a cinch project, with docs in
// the first commit and a second commit on top, and returns its path and SHA.
func fixture(t *testing.T) (string, string) {
	t.Helper()
	return fixtureOf(t, map[string]string{
		"AGENTS.md":               readFile(t, "testdata", "pin-AGENTS.md"),
		".docs/domain/rules.md":   "1. **TINY-001** The answer is forty-two.\n",
		".docs/workflows/flow.md": "workflow\n",
		".docs/architecture.md":   "arch\n",
	})
}

// agentFixture is shaped like DeltaDocs: docs in .agent/, agent tool config
// at the top, and an AGENTS.md that restates a rule under "Critical Rules".
func agentFixture(t *testing.T) (string, string) {
	t.Helper()
	return fixtureOf(t, map[string]string{
		"AGENTS.md":                 "# Agent guide\n\n## Critical Rules\n\n- Never commit to dev directly (GIT-001).\n\nSee .agent/conventions.md.\n",
		".agent/domain/rules.md":    "1. **TINY-001** The answer is forty-two.\n",
		".agent/workflows/flow.md":  "workflow\n",
		".claude/hooks/check.sh":    "#!/bin/sh\n",
		".cursor/.gitignore":        "x\n",
		".pi/extensions/auditor.ts": "export {}\n",
		".pi-lens.json":             "{}\n",
	})
}

// fixtureOf builds a tiny working repo with the given files plus the cinch
// scaffolding, the docs in the first commit and a second commit on top, and
// returns its path and the SHA of the tag on it.
func fixtureOf(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"cinch.yml":            "commit:\n  pattern: 'x'\n",
		".githooks/pre-commit": "#!/bin/sh\ncinch check\n",
		"scripts/cinch-log.sh": "#!/bin/sh\n",
		"src/main.go":          "package main\n",
		".gitignore":           "generated/\n",
	} {
		files[rel] = content
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.name", "T"}, {"config", "user.email", "t@example.invalid"},
		{"config", "commit.gpgsign", "false"}, {"add", "-A"}, {"commit", "-q", "-m", "first"},
	} {
		if _, err := git(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "src/main.go"), []byte("package main\n// two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"commit", "-q", "-am", "second"}, {"tag", "-a", "tiny/v1", "-m", "tiny"}} {
		if _, err := git(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	sha, err := resolveRef(dir, "tiny/v1")
	if err != nil {
		t.Fatal(err)
	}
	head, _ := git(dir, "rev-parse", "HEAD")
	if sha != strings.TrimSpace(head) {
		t.Fatalf("tag resolves to %s, HEAD is %s", sha, head)
	}
	return dir, sha
}

func needSandbox(t *testing.T) {
	t.Helper()
	if err := exec.Command(bwrapPath, "--ro-bind", "/usr", "/usr", "--symlink", "usr/bin", "/bin", "--symlink", "usr/lib", "/lib", "--symlink", "usr/lib", "/lib64", "--", "/bin/true").Run(); err != nil {
		t.Skipf("bubblewrap unusable: %v", err)
	}
}

const probeScript = `#!/bin/bash
echo "docs=$([ -d .docs ] && echo present || echo absent)"
echo "domain=$([ -f .docs/domain/rules.md ] && echo present || echo absent)"
echo "workflows=$([ -d .docs/workflows ] && echo present || echo absent)"
echo "cinchyml=$([ -e cinch.yml ] && echo present || echo absent)"
echo "githooks=$([ -e .githooks ] && echo present || echo absent)"
echo "cinch=$(command -v cinch || echo none)"
echo "cinch_find=$(find / -xdev -name cinch -type f 2>/dev/null | head -1)"
echo "commits=$(git rev-list --count --all)"
echo "git_history_docs=$(git log --all --name-only --format= | grep -c '^\.docs')"
echo "agents_docs=$(grep -c '\.docs' AGENTS.md)"
echo "agents_cinch=$(grep -ci 'cinch' AGENTS.md)"
echo "prompt_has_suffix=$(printf '%s' "$1" | grep -c 'Answer in at most five sentences')"
`

// runFake runs the whole pipeline with the fake engine over the tiny fixture.
func runFake(t *testing.T, script string, reps int) (out string, work string) {
	t.Helper()
	repo, sha := fixture(t)
	return runFakeOn(t, repo, sha, "", script, reps, false)
}

// runFakeOn runs the fake engine over repo with extra PINS lines.
func runFakeOn(t *testing.T, repo, sha, extraPins, script string, reps int, includeIgnored bool) (out string, work string) {
	t.Helper()
	needSandbox(t)
	tmp := t.TempDir()
	pins := "TINY_REF=tiny/v1\nTINY_SHA=" + sha + "\nSCHEDULE_SEED=5\nREPS=1\nWALL_CLOCK_MINUTES=1\n" + extraPins
	qs := `{"repo":"tiny","commit":"` + sha + `","questions":[
	  {"id":"TINY-001","doc":".docs/domain/rules.md","question":"What is the answer?","key":"The answer is forty-two.","notes":"","predict":"normative"},
	  {"id":"TINY-002","doc":".docs/domain/rules.md","question":"Another?","key":"Another rule key.","notes":"","predict":"recoverable"}]}`
	qs = strings.Replace(qs, `{"id":"TINY-002"`, `{"id":"TINY-IGN","doc":".docs/domain/rules.md","question":"An ignored one?","key":"Ignored rule key.","notes":"","predict":"normative","ignored":true},
	  {"id":"TINY-002"`, 1)
	paths := map[string]string{"PINS": pins, "q.json": qs, "agent.sh": script}
	for name, content := range paths {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out = filepath.Join(tmp, "results.csv")
	c := config{
		repo: repo, pinsPath: filepath.Join(tmp, "PINS"), questionsPath: filepath.Join(tmp, "q.json"),
		work: filepath.Join(tmp, "work"), out: out, engine: "fake", reps: reps, timeout: time.Minute,
		fakeScript: filepath.Join(tmp, "agent.sh"), armFilter: nil, includeIgnored: includeIgnored,
	}
	if err := run(c, map[string]bool{"gen": true, "objectbox": true}); err != nil {
		t.Fatal(err)
	}
	return out, tmp
}

func answers(t *testing.T, rows []result) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, r := range rows {
		facts := map[string]string{}
		for _, line := range strings.Split(r.Answer, "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				facts[k] = v
			}
		}
		out[r.Arm+"/"+r.QuestionID] = facts
	}
	return out
}

func TestArmProvisioningWithFakeEngine(t *testing.T) {
	// A cinch on the host's PATH must not reach either sandbox.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "cinch"), []byte("#!/bin/sh\necho host cinch\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	out, _ := runFake(t, probeScript, 1)
	rows, err := readCSV(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("%d rows, want 2 questions x 2 arms", len(rows))
	}
	got := answers(t, rows)
	for key, facts := range got {
		arm := strings.SplitN(key, "/", 2)[0]
		common := map[string]string{
			"workflows": "absent", "cinchyml": "absent", "githooks": "absent", "cinch": "none", "cinch_find": "",
			"commits": "1", "prompt_has_suffix": "1",
		}
		for k, v := range common {
			if facts[k] != v {
				t.Errorf("%s: %s=%q, want %q", key, k, facts[k], v)
			}
		}
		if arm == armCode {
			for k, v := range map[string]string{"docs": "absent", "domain": "absent", "git_history_docs": "0", "agents_docs": "0", "agents_cinch": "0"} {
				if facts[k] != v {
					t.Errorf("%s: %s=%q, want %q", key, k, facts[k], v)
				}
			}
		} else {
			for k, v := range map[string]string{"docs": "present", "domain": "present"} {
				if facts[k] != v {
					t.Errorf("%s: %s=%q, want %q", key, k, facts[k], v)
				}
			}
			if facts["agents_docs"] == "0" {
				t.Errorf("%s: the docs arm lost its pointer", key)
			}
		}
	}
	for _, r := range rows {
		if r.Status != "ok" || r.Turns != 1 || r.Question == "" || r.Key == "" || r.Predict == "" {
			t.Errorf("row %s: %+v", r.RunID, r)
		}
	}
}

func TestResumeKeepsVerdicts(t *testing.T) {
	out, _ := runFake(t, "#!/bin/bash\necho \"The answer is forty-two.\"\n", 1)
	if err := judgeAll(context.Background(), out, "", fakeJudge{}, 2, 7); err != nil {
		t.Fatal(err)
	}
	rows, err := readCSV(out)
	if err != nil {
		t.Fatal(err)
	}
	correct := 0
	for _, r := range rows {
		if r.Verdict == "" || r.JudgeModel != "fake" {
			t.Errorf("%s has no verdict", r.RunID)
		}
		if r.Verdict == "CORRECT" {
			correct++
		}
	}
	if correct != 2 { // TINY-001's key matches in both arms; TINY-002's does not
		t.Errorf("%d correct, want 2", correct)
	}
}

func TestParseVerdict(t *testing.T) {
	cases := []struct {
		reply, verdict, why string
		err                 bool
	}{
		{"CORRECT\nThe answer states the uniqueness rule.", "CORRECT", "The answer states the uniqueness rule.", false},
		{"VERDICT: PARTIAL - misses the exception", "PARTIAL", "misses the exception", false},
		{"**Verdict: WRONG**\nThe answer says case-insensitive.", "WRONG", "The answer says case-insensitive.", false},
		{"Reasoning first.\n\nABSTAIN: the answer says the repo does not settle it.", "ABSTAIN", "the answer says the repo does not settle it.", false},
		{"## CORRECT\nok", "CORRECT", "ok", false},
		{"The answer is INCORRECT in two ways.", "", "", true},
		{"WRONG\nAlso\nCORRECT", "", "", true},
		{"no verdict here, only a correct-sounding word", "", "", true},
		{"After reading: the answer is WRONG because x.", "WRONG", "After reading: the answer is WRONG because x.", false},
		{"The answer is CORRECT or WRONG, unclear", "", "", true},
	}
	for _, c := range cases {
		v, why, err := parseVerdict(c.reply)
		if (err != nil) != c.err || v != c.verdict || why != c.why {
			t.Errorf("parseVerdict(%q) = %q, %q, %v; want %q, %q, err=%t", c.reply, v, why, err, c.verdict, c.why, c.err)
		}
	}
}

func TestFillRubric(t *testing.T) {
	rubric := "Q: {{question}}\nA: {{answer}}\nKey: {{key}}\nNotes: {{notes}}\n"
	r := result{Repo: "zzrepo", Arm: "zzarm", Doc: "zzdoc.md", Question: "why?", Answer: "because {{key}}", Key: "KEY", Notes: "n"}
	got, err := fillRubric(rubric, r)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Q: why?\nA: because {{key}}\nKey: KEY\nNotes: n\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	for _, leak := range []string{"zzrepo", "zzarm", "zzdoc"} {
		if strings.Contains(got, leak) {
			t.Errorf("the judge's prompt names %s", leak)
		}
	}
	if _, err := fillRubric("{{question}} {{answer}} {{key}}", r); err == nil {
		t.Error("accepted a rubric without {{notes}}")
	}
}

func TestReplyText(t *testing.T) {
	got, err := replyText([]byte(`{"type":"result","subtype":"success","is_error":false,"result":"CORRECT\nfine"}`))
	if err != nil || got != "CORRECT\nfine" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := replyText([]byte(`{"type":"result","is_error":true,"result":"rate limited"}`)); err == nil {
		t.Error("an error result was accepted")
	}
	if _, err := replyText([]byte(`not json`)); err == nil {
		t.Error("garbage was accepted")
	}
}

type countingJudge struct {
	calls   int
	failFor map[string]bool
}

func (c *countingJudge) name() string { return "counting" }
func (c *countingJudge) judge(_ context.Context, _ string, r result) (string, string, error) {
	c.calls++
	if c.failFor[r.RunID] {
		return "", "", errors.New("unparseable")
	}
	return "PARTIAL", "stub", nil
}

func TestJudgeIsIdempotentAndResumable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.csv")
	rows := []result{
		{RunID: "A-code-r0", QuestionID: "A", Arm: "code", Question: "q", Key: "k", Answer: "a, with a comma\nand a \"quote\""},
		{RunID: "A-docs-r0", QuestionID: "A", Arm: "docs", Question: "q", Key: "k", Answer: "b"},
		{RunID: "B-docs-r0", QuestionID: "B", Arm: "docs", Question: "q", Key: "k", Answer: "", Status: "timeout"},
	}
	data, err := encodeCSV(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	j := &countingJudge{failFor: map[string]bool{"A-docs-r0": true}}
	if err := judgeAll(context.Background(), path, "r", j, 1, 7); err == nil {
		t.Fatal("a failed call was not reported")
	}
	got, _ := readCSV(path)
	if got[0].Verdict != "PARTIAL" || got[0].Answer != rows[0].Answer {
		t.Errorf("row 0: %+v", got[0])
	}
	if got[1].Verdict != "" || got[1].JudgeError == "" {
		t.Errorf("row 1 should be unjudged with an error: %+v", got[1])
	}
	if got[2].Verdict != "ABSTAIN" || j.calls != 3 {
		t.Errorf("an empty answer is ABSTAIN without a call, a failure is retried once: %+v, %d calls", got[2], j.calls)
	}
	j.failFor = nil
	j.calls = 0
	if err := judgeAll(context.Background(), path, "r", j, 1, 7); err != nil {
		t.Fatal(err)
	}
	if j.calls != 1 {
		t.Errorf("resume made %d calls, want 1", j.calls)
	}
	j.calls = 0
	if err := judgeAll(context.Background(), path, "r", j, 1, 7); err != nil || j.calls != 0 {
		t.Errorf("a finished file was judged again: %v, %d calls", err, j.calls)
	}
}

func TestFakeJudge(t *testing.T) {
	r := result{Key: "Category names are unique.", Answer: "Yes: Category names are unique, always."}
	if v, _, _ := (fakeJudge{}).judge(context.Background(), "", r); v != "CORRECT" {
		t.Errorf("got %s", v)
	}
	r.Answer = "No idea."
	if v, _, _ := (fakeJudge{}).judge(context.Background(), "", r); v != "WRONG" {
		t.Errorf("got %s", v)
	}
}

func TestWilson(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 5e-4 }
	cases := []struct {
		k, n      int
		low, high float64
	}{{5, 10, 0.2366, 0.7634}, {0, 10, 0, 0.2775}, {10, 10, 0.7225, 1}, {3, 20, 0.0524, 0.3601}}
	for _, c := range cases {
		low, high := wilson(c.k, c.n)
		if !near(low, c.low) || !near(high, c.high) {
			t.Errorf("wilson(%d, %d) = %.4f, %.4f; want %.4f, %.4f", c.k, c.n, low, high, c.low, c.high)
		}
	}
	if low, high := wilson(0, 0); low != 0 || high != 0 {
		t.Error("n=0 must be 0, 0")
	}
}

func verdictRows(repo, id, doc, predict string, code, docs []string) []result {
	var rows []result
	for i, v := range code {
		rows = append(rows, result{Repo: repo, Commit: "c", QuestionID: id, Doc: doc, Predict: predict, Arm: armCode, Rep: i, Verdict: v})
	}
	for i, v := range docs {
		rows = append(rows, result{Repo: repo, Commit: "c", QuestionID: id, Doc: doc, Predict: predict, Arm: armDocs, Rep: i, Verdict: v})
	}
	return rows
}

func TestSummarize(t *testing.T) {
	var rows []result
	C, P, W, A := "CORRECT", "PARTIAL", "WRONG", "ABSTAIN"
	add := func(id, doc, predict string, code, docs []string) {
		rows = append(rows, verdictRows("r", id, doc, predict, code, docs)...)
	}
	add("R-1", "a.md", "recoverable", []string{C, C, W}, []string{C, C, C}) // recoverable
	add("N-1", "a.md", "normative", []string{W, A, C}, []string{C, C, P})   // normative (1/3 code, 2/3 docs)
	add("N-2", "b.md", "normative", []string{W, W, W}, []string{C, C, C})   // normative
	add("N-3", "b.md", "recoverable", []string{P, P, W}, []string{C, C, W}) // normative; PARTIAL is not correct
	add("B-1", "b.md", "normative", []string{C, C, C}, []string{W, C, A})   // bad question, even though code is right
	add("I-1", "b.md", "normative", []string{C, C, C}, []string{C, C, ""})  // incomplete: one docs run unjudged
	sums := summarize(rows, 3, 2)
	if len(sums) != 1 {
		t.Fatalf("%d repos", len(sums))
	}
	s := sums[0]
	classes := map[string]string{}
	for _, r := range s.Rules {
		classes[r.ID] = r.Class
	}
	want := map[string]string{"R-1": classRecoverable, "N-1": classNormative, "N-2": classNormative, "N-3": classNormative, "B-1": classBad, "I-1": classIncomplete}
	if !reflect.DeepEqual(classes, want) {
		t.Errorf("classes = %v, want %v", classes, want)
	}
	if s.Valid != 4 || s.Recoverable != 1 || s.Normative != 3 || s.Bad != 1 || s.Incomplete != 1 {
		t.Errorf("counts: %+v", s)
	}
	if !reflect.DeepEqual(s.NormativeID, []string{"N-1", "N-2", "N-3"}) {
		t.Errorf("normative ids = %v", s.NormativeID)
	}
	low, high := wilson(3, 4)
	if s.Share != 0.75 || s.CILow != low || s.CIHigh != high {
		t.Errorf("share %v [%v, %v]", s.Share, s.CILow, s.CIHigh)
	}
	if len(s.ByDoc) != 2 || s.ByDoc[0].Doc != "a.md" || s.ByDoc[0].Valid != 2 || s.ByDoc[0].Normative != 1 ||
		s.ByDoc[1].Valid != 2 || s.ByDoc[1].Normative != 2 || s.ByDoc[1].Bad != 1 {
		t.Errorf("by doc: %+v", s.ByDoc)
	}
	// Predictions on valid rules: R-1 (recoverable, ok), N-1 ok, N-2 ok, N-3 predicted recoverable but observed normative.
	if s.Predictions.Agree != 3 || s.Predictions.Disagree != 1 {
		t.Errorf("predictions: %+v", s.Predictions)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	in := []result{{RunID: "a", Repo: "r", Rep: 2, Seed: 9, WallMS: 7000, CostUSD: 0.5, TimedOut: true, Question: "q,\"x\"\nline", Answer: "multi\nline", Verdict: "WRONG", Justification: "why"}}
	data, err := encodeCSV(in)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "r.csv")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := readCSV(path)
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Errorf("round trip: %v\n%+v\n%+v", err, in, out)
	}
}

func TestBuildPrompt(t *testing.T) {
	got := buildPrompt("How many?\n")
	if !strings.HasPrefix(got, "How many?\n\nAnswer in at most five sentences. State the specific behavior (numbers, conditions, who may do what). Cite the file and line you based it on. If the repository does not settle it, say so.") {
		t.Errorf("prompt = %q", got)
	}
}

func TestPinsRefuseTODO(t *testing.T) {
	pins, err := loadPins(filepath.Join("testdata", "PINS.todo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, repo := range []string{"budgetron", "deltadocs"} {
		if _, _, err := fixturePin(pins, repo); err == nil || !strings.Contains(err.Error(), "TODO") {
			t.Errorf("%s: a TODO pin was accepted (err %v)", repo, err)
		}
	}
	if _, _, err := fixturePin(pins, "missing"); err == nil {
		t.Error("a repo without pins was accepted")
	}
	if ref, sha, err := fixturePin(map[string]string{"X_Y_REF": "r", "X_Y_SHA": "s"}, "x-y"); err != nil || ref != "r" || sha != "s" {
		t.Errorf("got %q %q %v", ref, sha, err)
	}
}

func TestArmSpecFromPins(t *testing.T) {
	pins, err := loadPins("PINS")
	if err != nil {
		t.Fatal(err)
	}
	b, err := armSpecFor(pins, ".", "budgetron")
	if err != nil || b.docsRoot != ".docs" || len(b.strip) != 0 || b.agents != nil {
		t.Errorf("budgetron: %+v %v", b, err)
	}
	d, err := armSpecFor(pins, ".", "deltadocs")
	if err != nil || d.docsRoot != ".agent" || !reflect.DeepEqual(d.strip, []string{".claude", ".cursor", ".pi", ".pi-lens.json"}) {
		t.Errorf("deltadocs: %+v %v", d, err)
	}
	// The replacement texts are DESIGN's.
	if !strings.HasSuffix(d.agents[armCode], "## Project\n\nDeltaDocs is a full-stack app in a monorepo: a Go/PostgreSQL backend in `backend/` and a SvelteKit/DaisyUI frontend (Bun, Vite) in `frontend/`.\n") ||
		strings.Contains(d.agents[armCode], ".agent") || strings.Contains(d.agents[armCode], "GIT-001") {
		t.Errorf("code AGENTS.md:\n%s", d.agents[armCode])
	}
	if !strings.Contains(d.agents[armDocs], "docs live in `.agent/`") || strings.Contains(d.agents[armDocs], "cinch check") ||
		!strings.HasSuffix(d.agents[armDocs], strings.TrimPrefix(d.agents[armCode], "# Agent instructions\n\n")) {
		t.Errorf("docs AGENTS.md:\n%s", d.agents[armDocs])
	}
}

func TestScrubAnswer(t *testing.T) {
	cases := map[string]string{
		"See .docs/domain/categories.md:12 for the rule.":                     "See [ref]:12 for the rule.",
		"Per /work/.agent/conventions.md line 3, never commit.":               "Per [ref] line 3, never commit.",
		"The docs say names are unique (lib/db/category_repository.dart:40).": "The [ref] say names are unique (lib/db/category_repository.dart:40).",
		"AGENTS.md and README.md mention it; see docs/x.md":                   "[ref] and [ref] mention it; see [ref]",
		"It is Documented in the Documentation (DOCS).":                       "It is [ref] in the [ref] ([ref]).",
		"Code: lib/models/money.dart:7 and test/money_test.dart (MONEY-001)":  "Code: lib/models/money.dart:7 and test/money_test.dart (MONEY-001)",
		"The user.agent header and a document are untouched; docstring too.":  "The user.agent header and a document are untouched; docstring too.",
		"Look in ./.docs/ and .agent/api/x.md, and .docs":                     "Look in [ref] and [ref], and [ref]",
	}
	for in, want := range cases {
		if got := scrubAnswer(in); got != want {
			t.Errorf("scrubAnswer(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// seeingJudge records the answer a judge was shown.
type seeingJudge struct{ saw map[string]string }

func (s *seeingJudge) name() string { return "seeing" }
func (s *seeingJudge) judge(_ context.Context, _ string, r result) (string, string, error) {
	s.saw[r.RunID] = r.Answer
	return "CORRECT", "ok", nil
}

func TestJudgeIsBlinded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.csv")
	docsAnswer := "Names are unique per .docs/domain/categories.md:5 and the docs say so; also lib/db/category_repository.dart:40."
	codeAnswer := "Names are unique, see lib/db/category_repository.dart:40."
	rows := []result{
		{RunID: "A-docs-r0", QuestionID: "A", Arm: armDocs, Question: "q", Key: "k", Answer: docsAnswer},
		{RunID: "A-code-r0", QuestionID: "A", Arm: armCode, Question: "q", Key: "k", Answer: codeAnswer},
		{RunID: "B-docs-r0", QuestionID: "B", Arm: armDocs, Question: "q", Key: "k", Answer: "Per AGENTS.md and .agent/api/x.md it is 5."},
	}
	data, _ := encodeCSV(rows)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	j := &seeingJudge{saw: map[string]string{}}
	if err := judgeAll(context.Background(), path, "r", j, 1, 3); err != nil {
		t.Fatal(err)
	}
	for id, saw := range j.saw {
		for _, leak := range []string{".docs", ".agent", ".md", "docs say", "AGENTS"} {
			if strings.Contains(saw, leak) {
				t.Errorf("%s: the judge saw %q in %q", id, leak, saw)
			}
		}
	}
	if j.saw["A-code-r0"] != codeAnswer {
		t.Errorf("a code-arm answer was altered: %q", j.saw["A-code-r0"])
	}
	if want := "Names are unique per [ref]:5 and the [ref] say so; also lib/db/category_repository.dart:40."; j.saw["A-docs-r0"] != want {
		t.Errorf("docs-arm scrub = %q, want %q", j.saw["A-docs-r0"], want)
	}
	got, _ := readCSV(path)
	for i, r := range got {
		if r.Answer != rows[i].Answer {
			t.Errorf("%s: the stored answer was altered: %q", r.RunID, r.Answer)
		}
		if r.JudgedAnswer != j.saw[r.RunID] {
			t.Errorf("%s: judged_answer %q, the judge saw %q", r.RunID, r.JudgedAnswer, j.saw[r.RunID])
		}
	}
}

func TestAgentRootedProvisioning(t *testing.T) {
	repo, sha := agentFixture(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "arms"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"code.md", "docs.md"} {
		if err := os.WriteFile(filepath.Join(tmp, "arms", name), []byte(readFile(t, "testdata", "arms", name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := strings.ReplaceAll(probeScript, ".docs", ".agent") + `
echo "claude=$([ -e .claude ] && echo present || echo absent)"
echo "cursor=$([ -e .cursor ] && echo present || echo absent)"
echo "pi=$([ -e .pi ] && echo present || echo absent)"
echo "pilens=$([ -e .pi-lens.json ] && echo present || echo absent)"
echo "critical=$(grep -c 'Critical Rules' AGENTS.md)"
echo "gitrule=$(grep -c 'GIT-001' AGENTS.md)"
echo "history_claude=$(git log --all --name-only --format= | grep -c '^\.claude')"
`
	// The arms directory is found relative to PINS, which runFakeOn writes into
	// its own temp dir; an absolute path keeps it simple.
	extra := "TINY_DOCS=.agent\nTINY_STRIP=.claude,.cursor,.pi,.pi-lens.json\nTINY_AGENTS_DIR=" + filepath.Join(tmp, "arms") + "\n"
	out, _ := runFakeOn(t, repo, sha, extra, script, 1, false)
	rows, err := readCSV(out)
	if err != nil || len(rows) != 4 {
		t.Fatalf("%d rows, %v", len(rows), err)
	}
	for key, facts := range answers(t, rows) {
		arm := strings.SplitN(key, "/", 2)[0]
		want := map[string]string{
			"workflows": "absent", "cinchyml": "absent", "githooks": "absent", "cinch": "none", "commits": "1",
			"claude": "absent", "cursor": "absent", "pi": "absent", "pilens": "absent", "critical": "0", "gitrule": "0",
			"history_claude": "0",
		}
		if arm == armCode {
			want["docs"], want["domain"], want["agents_docs"] = "absent", "absent", "0"
		} else {
			want["docs"], want["domain"] = "present", "present"
		}
		for k, v := range want {
			if facts[k] != v {
				t.Errorf("%s: %s=%q, want %q", key, k, facts[k], v)
			}
		}
		if arm == armDocs && facts["agents_docs"] == "0" {
			t.Errorf("%s: the docs arm lost its pointer", key)
		}
	}
}

func TestIncludeIgnored(t *testing.T) {
	f, err := parseQuestions([]byte(`{"repo":"r","commit":"c","questions":[
	  {"id":"A","doc":"d","question":"q","key":"k","predict":"normative"},
	  {"id":"B","doc":"d","question":"q","key":"k","predict":"recoverable","ignored":true},
	  {"id":"C","doc":"d","question":"","key":"","ignored":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if qs, err := f.selected(false); err != nil || len(qs) != 1 {
		t.Errorf("default: %v %v", qs, err)
	}
	if _, err := f.selected(true); err == nil {
		t.Error("an incomplete ignored question was scheduled")
	}
	f.Questions = f.Questions[:2]
	if qs, err := f.selected(true); err != nil || len(qs) != 2 || qs[1].ID != "B" {
		t.Errorf("included: %v %v", qs, err)
	}
	// End to end: the ignored question runs, and is marked so in the CSV.
	repo, sha := fixture(t)
	out, _ := runFakeOn(t, repo, sha, "", "#!/bin/bash\necho hi\n", 1, true)
	rows, err := readCSV(out)
	if err != nil || len(rows) != 6 {
		t.Fatalf("%d rows, %v", len(rows), err)
	}
	for _, r := range rows {
		if r.Ignored != (r.QuestionID == "TINY-IGN") {
			t.Errorf("%s ignored=%t", r.RunID, r.Ignored)
		}
	}
}

func TestSummarizeIgnoredRow(t *testing.T) {
	C, W := "CORRECT", "WRONG"
	var rows []result
	rows = append(rows, verdictRows("r", "N-1", "a.md", "normative", []string{W, W, W}, []string{C, C, C})...)
	ign := verdictRows("r", "G-1", "a.md", "normative", []string{W, W, W}, []string{C, C, C})
	ign2 := verdictRows("r", "G-2", "a.md", "recoverable", []string{C, C, C}, []string{C, C, C})
	for i := range ign {
		ign[i].Ignored = true
	}
	for i := range ign2 {
		ign2[i].Ignored = true
	}
	rows = append(append(rows, ign...), ign2...)
	s := summarize(rows, 3, 2)[0]
	if s.Valid != 1 || s.Normative != 1 || !reflect.DeepEqual(s.NormativeID, []string{"N-1"}) || len(s.ByDoc) != 1 || s.ByDoc[0].Valid != 1 {
		t.Errorf("the primary share counted ignored rules: %+v", s)
	}
	if s.Ignored == nil || s.Ignored.Rules != 2 || s.Ignored.Valid != 2 || s.Ignored.Normative != 1 || !reflect.DeepEqual(s.Ignored.NormativeID, []string{"G-1"}) {
		t.Errorf("ignored row: %+v", s.Ignored)
	}
	var buf strings.Builder
	printSummary(&buf, s)
	if !strings.Contains(buf.String(), "ignored rules (own row, outside the share)") || strings.Contains(buf.String(), "normative bound rules (2)") {
		t.Errorf("printout:\n%s", buf.String())
	}
}

func TestParsePiTranscript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	lines := strings.Join([]string{
		`{"type":"tool_execution_start","toolCallId":"1","toolName":"bash","args":{"command":"ls"}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"looking"}],"usage":{"input":10,"output":2,"cacheRead":1,"cacheWrite":0,"cost":{"total":0.5}}}}`,
		`{"type":"message_end","message":{"role":"user","content":[]}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"It is X (a.go:3)."}],"usage":{"input":20,"output":5}}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := parseTranscript("pi", path)
	if err != nil {
		t.Fatal(err)
	}
	if tr.FinalText != "It is X (a.go:3)." || tr.Turns != 2 || tr.Input != 30 || tr.Output != 7 || tr.ToolCalls != 1 || tr.CostUSD != 0.5 {
		t.Errorf("trace = %+v", tr)
	}
	// A run cut off mid-tool-call has no answer, not its earlier musing.
	cut := lines + "\n" + `{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall"}]}}`
	if err := os.WriteFile(path, []byte(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	if tr, _ := parseTranscript("pi", path); tr.FinalText != "" {
		t.Errorf("final text = %q", tr.FinalText)
	}
}

// TestBudgetronDryRun is the end-to-end dry run on the scrubbed Budgetron
// mirror (it runs make gen, so it is opt-in): RECOVER_E2E=1 go test -run Budgetron.
func TestBudgetronDryRun(t *testing.T) {
	if os.Getenv("RECOVER_E2E") == "" {
		t.Skip("set RECOVER_E2E=1 to run the Budgetron dry run")
	}
	needSandbox(t)
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	out := filepath.Join(t.TempDir(), "results.csv")
	_ = os.RemoveAll(filepath.Join(cache, "cinch-recover-e2e", "runs")) // earlier runs resume, and would add rows
	c := config{
		repo: filepath.Join(home, "code", "budgetron.scrubbed.git"), pinsPath: filepath.Join("testdata", "PINS.dryrun"),
		questionsPath: filepath.Join("testdata", "dryrun-questions.json"), work: filepath.Join(cache, "cinch-recover-e2e"),
		out: out, engine: "fake", reps: 1, timeout: 2 * time.Minute, fakeScript: filepath.Join("testdata", "fake-answer.sh"),
		flutter: filepath.Join(home, "dev", "flutter"), gen: "make gen", objectbox: filepath.Join(home, "code", "budgetron", ".objectbox", "lib"),
	}
	if err := run(c, map[string]bool{"gen": true, "objectbox": true}); err != nil {
		t.Fatal(err)
	}
	rows, err := readCSV(out)
	if err != nil || len(rows) != 4 {
		t.Fatalf("%d rows, %v", len(rows), err)
	}
}

func TestRuleText(t *testing.T) {
	cases := map[string]string{
		"1. **GIT-001** Never commit to `dev`. <!-- cinch:ignore: process convention, see (.claude/x.sh) -->": "Never commit to `dev`.",
		"12. **ACCT-001** Balance equals opening plus the sum.":                                               "Balance equals opening plus the sum.",
		"**X-1** No number.": "No number.",
		"Plain text.":        "Plain text.",
		"3. **A-3** Two\n   lines <!-- cinch:ignore: a\nb -->": "Two\n   lines",
	}
	for in, want := range cases {
		if got := ruleText(in); got != want {
			t.Errorf("ruleText(%q) = %q, want %q", in, got, want)
		}
	}
}

func realQuestionFiles(t *testing.T) []*questionFile {
	t.Helper()
	var files []*questionFile
	for _, name := range []string{"budgetron.json", "deltadocs.json"} {
		path := filepath.Join("questions", name)
		if _, err := os.Stat(path); err != nil {
			t.Skipf("%s is not here (DeltaDocs' questions may be kept out of the repo)", path)
		}
		f, err := loadQuestions(path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

func TestProbesMatchDesign(t *testing.T) {
	probes, err := loadProbes("calibration-probes.json")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(readFile(t, "DESIGN.md"), "\n")
	var table [][3]string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "| Probe |") {
			in = true
			continue
		}
		if !in || strings.HasPrefix(l, "|---") {
			continue
		}
		if !strings.HasPrefix(l, "|") {
			break
		}
		cells := strings.Split(strings.Trim(l, "| "), " | ")
		if len(cells) != 3 {
			t.Fatalf("probe row %q", l)
		}
		table = append(table, [3]string{cells[0], strings.Trim(cells[1], `"`), cells[2]})
	}
	if len(table) != 14 || len(probes) != 14 {
		t.Fatalf("%d table rows, %d probes, want 14", len(table), len(probes))
	}
	for i, p := range probes {
		if p.Rule != table[i][0] || p.Answer != table[i][1] || p.Expected != table[i][2] {
			t.Errorf("probe %d differs from DESIGN: %+v vs %v", i, p, table[i])
		}
	}
}

func TestBuildCalibration(t *testing.T) {
	files := realQuestionFiles(t)
	probes, err := loadProbes("calibration-probes.json")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := buildCalibration(files, probes, 20261101)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	seen := map[string]bool{}
	for _, r := range rows {
		count[r.Set]++
		if seen[r.key()] {
			t.Errorf("duplicate row %s", r.key())
		}
		seen[r.key()] = true
		if r.input.Question == "" || r.input.Key == "" || r.input.Answer != r.Answer {
			t.Errorf("row %s lacks its inputs", r.key())
		}
		if r.Set == setOwnText && (strings.Contains(r.Answer, "cinch:ignore") || strings.HasPrefix(r.Answer, "1.") || strings.HasPrefix(r.Answer, "**")) {
			t.Errorf("%s: the own text keeps its scaffolding: %.60q", r.ID, r.Answer)
		}
	}
	if count[setOwnText] != 132 || count[setAbstain] != 10 || count[setProbe] != 14 {
		t.Errorf("counts = %v, want 132 / 10 / 14", count)
	}
	again, _ := buildCalibration(files, probes, 20261101)
	other, _ := buildCalibration(files, probes, 1)
	pick := func(rs []calRow) (ids []string) {
		for _, r := range rs {
			if r.Set == setAbstain {
				ids = append(ids, r.ID)
			}
		}
		return
	}
	if !reflect.DeepEqual(pick(rows), pick(again)) || reflect.DeepEqual(pick(rows), pick(other)) {
		t.Error("set (b) must depend on the seed, and only on the seed")
	}
	if _, err := buildCalibration(files, []probe{{ID: "X", Rule: "NOPE-1", Repo: "budgetron", Answer: "a", Expected: "WRONG"}}, 1); err == nil {
		t.Error("a probe for an unknown rule was accepted")
	}
}

func TestCalibrateWithFakeJudge(t *testing.T) {
	files := realQuestionFiles(t)
	probes, _ := loadProbes("calibration-probes.json")
	rows, err := buildCalibration(files, probes, 20261101)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "calibration-fake.csv")
	var out strings.Builder
	err = calibrate(context.Background(), &out, path, "", fakeJudge{}, rows, 2, 7)
	// The fake judge cannot give PARTIAL or tell a negation probe, so (c) fails; (a) and (b) pass.
	if err == nil {
		t.Fatal("the fake judge passed the probes")
	}
	for _, want := range []string{"PASS  (a) own text judged CORRECT", "132/132", "PASS  (b) fixed abstain sentence judged ABSTAIN", "10/10", "FAIL  (c)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in\n%s", want, out.String())
		}
	}
	got, err := readCalCSV(path)
	if err != nil || len(got) != 156 {
		t.Fatalf("%d rows in the CSV, %v", len(got), err)
	}
	header := strings.SplitN(readFile(t, path), "\n", 2)[0]
	if !strings.HasPrefix(header, "id,set,answer,expected,verdict,justification") {
		t.Errorf("header %q", header)
	}
}

// oracle answers every calibration row with its expected verdict.
type oracle struct {
	expected map[string]string
	flip     map[string]string // answer -> verdict to give instead
	calls    int
	mu       sync.Mutex
}

func (o *oracle) name() string { return "oracle" }
func (o *oracle) judge(_ context.Context, _ string, r result) (string, string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if v, ok := o.flip[r.Answer]; ok {
		return v, "flipped", nil
	}
	return o.expected[r.Answer], "oracle", nil
}

func newOracle(rows []calRow) *oracle {
	o := &oracle{expected: map[string]string{}, flip: map[string]string{}}
	for _, r := range rows {
		o.expected[scrubAnswer(r.Answer)] = r.Expected
	}
	return o
}

func TestCalibrateThresholdsAndResume(t *testing.T) {
	files := realQuestionFiles(t)
	probes, _ := loadProbes("calibration-probes.json")
	build := func() []calRow {
		rows, err := buildCalibration(files, probes, 20261101)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	run := func(path string, o *oracle) (string, error) {
		var out strings.Builder
		err := calibrate(context.Background(), &out, path, "", o, build(), 1, 7)
		return out.String(), err
	}
	// All as expected: passes, and a second run makes no calls.
	path := filepath.Join(t.TempDir(), "c.csv")
	o := newOracle(build())
	if out, err := run(path, o); err != nil || !strings.Contains(out, "calibration passed") {
		t.Fatalf("err %v\n%s", err, out)
	}
	first := o.calls
	if first == 0 {
		t.Fatal("no judge calls")
	}
	if _, err := run(path, o); err != nil || o.calls != first {
		t.Errorf("resume made %d new calls (err %v)", o.calls-first, err)
	}
	// 129 of 132 is 97.7%: still a pass. 128 of 132 is 96.97%: a fail.
	rows := build()
	var own []string
	for _, r := range rows {
		if r.Set == setOwnText {
			own = append(own, scrubAnswer(r.Answer))
		}
	}
	for flips, wantErr := range map[int]bool{3: false, 4: true} {
		o := newOracle(rows)
		for _, a := range own[:flips] {
			o.flip[a] = "WRONG"
		}
		out, err := run(filepath.Join(t.TempDir(), "c.csv"), o)
		if (err != nil) != wantErr {
			t.Errorf("%d own-text misses: err %v\n%s", flips, err, out)
		}
	}
	// One abstain answer judged otherwise fails (b).
	o = newOracle(rows)
	o.flip[abstainSentence] = "WRONG"
	if out, err := run(filepath.Join(t.TempDir(), "c.csv"), o); err == nil || !strings.Contains(out, "FAIL  (b)") {
		t.Errorf("(b) did not fail: %v\n%s", err, out)
	}
	// A probe at the wrong verdict fails (c) and is named.
	o = newOracle(rows)
	o.flip[probes[5].Answer] = "WRONG"
	o.flip[probes[0].Answer] = "WRONG"
	if out, err := run(filepath.Join(t.TempDir(), "c.csv"), o); err == nil || !strings.Contains(out, "P01 (WRONG, want CORRECT)") {
		t.Errorf("(c) did not name the probe: %v\n%s", err, out)
	}
}

func TestCalibrateUnjudgedFails(t *testing.T) {
	files := realQuestionFiles(t)
	probes, _ := loadProbes("calibration-probes.json")
	rows, _ := buildCalibration(files, probes, 1)
	failing := failingJudge{}
	var out strings.Builder
	path := filepath.Join(t.TempDir(), "c.csv")
	if err := calibrate(context.Background(), &out, path, "", failing, rows, 1, 1); err == nil || !strings.Contains(out.String(), "no verdict") {
		t.Errorf("err %v\n%s", err, out.String())
	}
}

type failingJudge struct{}

func (failingJudge) name() string { return "failing" }
func (failingJudge) judge(context.Context, string, result) (string, string, error) {
	return "", "", errors.New("down")
}
