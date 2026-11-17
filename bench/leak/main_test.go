package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ----- helpers -----

func testPins(t *testing.T) map[string]string {
	t.Helper()
	pins, err := loadPins("PINS")
	if err != nil {
		t.Fatal(err)
	}
	return pins
}

func testHarness(t *testing.T, mutate func(*config)) *harness {
	t.Helper()
	c := config{pinsPath: "PINS", leakDir: ".", engine: "pi"}
	if mutate != nil {
		mutate(&c)
	}
	h, err := newHarness(c)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func ids(jobs []job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.id()
	}
	return out
}

func countBy(jobs []job, key func(job) string) map[string]int {
	out := map[string]int{}
	for _, j := range jobs {
		out[key(j)]++
	}
	return out
}

// ----- schedule -----

func TestScheduleIsDeterministicAndComplete(t *testing.T) {
	h := testHarness(t, nil)
	a := schedule(h.seed, "pi", func(task string) []int { return armsOf(h.pins, task) }, h.sizes)
	b := schedule(h.seed, "pi", func(task string) []int { return armsOf(h.pins, task) }, h.sizes)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two calls with the same seed and engine gave different orders")
	}
	// T1-T3 x (15 + 5 + 15) and T4 x (15 + 15): the owner runs T4 on arms 1 and 3 only
	if len(a) != 135 {
		t.Fatalf("full schedule has %d runs, want 135", len(a))
	}
	for i, j := range a {
		if j.Position != i {
			t.Fatalf("job %d has position %d", i, j.Position)
		}
	}
	other := schedule(h.seed, "claude", func(task string) []int { return armsOf(h.pins, task) }, h.sizes)
	if reflect.DeepEqual(ids(a), ids(other)) {
		t.Fatal("pi and claude got the same order")
	}
	if got := schedule(h.seed+1, "pi", func(task string) []int { return armsOf(h.pins, task) }, h.sizes); reflect.DeepEqual(ids(a), ids(got)) {
		t.Fatal("a different seed gave the same order")
	}
}

// The registered order: a change here changes the pre-registration, so the
// first slots of the pi schedule are pinned.
func TestScheduleGoldenPositions(t *testing.T) {
	h := testHarness(t, nil)
	all := schedule(h.seed, "pi", func(task string) []int { return armsOf(h.pins, task) }, h.sizes)
	want := map[int]string{0: "T1-arm3-r9", 3: "T4-arm3-r1", 8: "T4-arm1-r14"}
	for pos, id := range want {
		if all[pos].id() != id {
			t.Errorf("position %d is %s, registered %s", pos, all[pos].id(), id)
		}
	}
}

func TestStageSelection(t *testing.T) {
	h := testHarness(t, nil)
	armsOfTask := func(task string) []int { return armsOf(h.pins, task) }
	all := schedule(h.seed, "pi", armsOfTask, h.sizes)
	s1 := selectJobs(all, 1, h.sizes, nil, nil)
	s2 := selectJobs(all, 2, h.sizes, nil, nil)
	if len(s1) != 79 || len(s2) != 56 {
		t.Fatalf("stage 1 has %d runs and stage 2 has %d, want 79 and 56", len(s1), len(s2))
	}
	cell := func(j job) string { return fmt.Sprintf("%s/%d", j.Task, j.Arm) }
	if _, has := countBy(s1, cell)["T4/2"]; has {
		t.Error("T4 has arm 2 runs, but PINS says T4_ARMS=1,3")
	}
	for key, n := range countBy(s1, cell) {
		want := 8
		if strings.HasSuffix(key, "/2") {
			want = 5
		}
		if n != want {
			t.Errorf("stage 1 cell %s has %d runs, want %d", key, n, want)
		}
	}
	for key, n := range countBy(s2, cell) {
		if strings.HasSuffix(key, "/2") {
			t.Errorf("stage 2 has arm 2 runs in %s", key)
		}
		if n != 7 {
			t.Errorf("stage 2 cell %s has %d runs, want 7", key, n)
		}
	}
	// A subset keeps its slots in the full order.
	sub := selectJobs(all, 1, h.sizes, []string{"T3"}, []int{1, 3})
	last := -1
	for _, j := range sub {
		if all[j.Position].id() != j.id() || j.Position <= last {
			t.Fatalf("subset job %s lost its slot", j.id())
		}
		last = j.Position
	}
	if len(sub) != 16 {
		t.Errorf("T3 arms 1,3 stage 1 has %d runs, want 16", len(sub))
	}
}

func TestStageSizeOverride(t *testing.T) {
	h := testHarness(t, func(c *config) { c.perStage = 3 })
	all := schedule(h.seed, "pi", func(task string) []int { return armsOf(h.pins, task) }, h.sizes)
	s1 := selectJobs(all, 1, h.sizes, []string{"T1"}, []int{1})
	s2 := selectJobs(all, 2, h.sizes, []string{"T1"}, []int{1})
	if len(s1) != 3 || len(s2) != 12 {
		t.Fatalf("-n 3 gave %d and %d runs for T1 arm 1, want 3 and 12", len(s1), len(s2))
	}
}

func TestPilotSchedule(t *testing.T) {
	pi := testHarness(t, func(c *config) { c.engine = "pi"; c.pilot = true; c.pilotN1, c.pilotN3 = -1, -1 })
	jobs := pi.jobsFor()
	if len(jobs) != 16 {
		t.Fatalf("pi pilot has %d runs, want 16 (3 on arm 1 and 1 on arm 3 for each of four tasks)", len(jobs))
	}
	cell := func(j job) string { return fmt.Sprintf("%s/%d", j.Task, j.Arm) }
	for key, n := range countBy(jobs, cell) {
		want := 3
		if strings.HasSuffix(key, "/3") {
			want = 1
		}
		if n != want {
			t.Errorf("pilot cell %s has %d runs, want %d", key, n, want)
		}
	}
	for _, j := range jobs {
		if !j.Pilot {
			t.Errorf("%s is not marked as a pilot run", j.id())
		}
	}
	if !reflect.DeepEqual(ids(jobs), ids(testHarness(t, func(c *config) { c.engine = "pi"; c.pilot = true; c.pilotN1, c.pilotN3 = -1, -1 }).jobsFor())) {
		t.Error("the pilot order is not deterministic")
	}
	claude := testHarness(t, func(c *config) { c.engine = "claude"; c.pilot = true; c.pilotN1, c.pilotN3 = -1, -1 })
	if got := claude.jobsFor(); len(got) != 4 {
		t.Errorf("claude pilot has %d runs, want 4 (one arm 1 run per task)", len(got))
	}
	// The pilot never shares an order with the study's.
	study := testHarness(t, nil)
	if pilotOrder := ids(jobs); reflect.DeepEqual(pilotOrder[:4], ids(study.jobsFor())[:4]) {
		t.Error("pilot and study start the same")
	}
}

func TestCalibrationEnginesRunOnePerTaskAndArm(t *testing.T) {
	h := testHarness(t, func(c *config) { c.engine = "none"; c.taskFilter = []string{"T1", "T3"}; c.armFilter = []int{1, 3} })
	if got := ids(h.jobsFor()); !reflect.DeepEqual(got, []string{"T1-arm1-r0", "T1-arm3-r0", "T3-arm1-r0", "T3-arm3-r0"}) {
		t.Errorf("calibration jobs: %v", got)
	}
}

func TestT4ArmsFromPins(t *testing.T) {
	pins := map[string]string{"T4_ARMS": "1,3"}
	if got := armsOf(pins, "T4"); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Errorf("T4 arms: %v", got)
	}
	if got := armsOf(pins, "T1"); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("T1 arms: %v", got)
	}
}

// ----- pins -----

func TestPinsRefuseTODO(t *testing.T) {
	pins, err := loadPins("testdata/PINS.todo")
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePins(pins, "CINCH_VERSION", "CAP_T2"); err != nil {
		t.Errorf("filled pins refused: %v", err)
	}
	err = requirePins(pins, "CAP_T1", "T1_ARM1_TREE_SHA256", "CAP_T9")
	if err == nil || !strings.Contains(err.Error(), "CAP_T1") || !strings.Contains(err.Error(), "T1_ARM1_TREE_SHA256") || !strings.Contains(err.Error(), "CAP_T9") {
		t.Errorf("want a refusal naming CAP_T1, T1_ARM1_TREE_SHA256 and CAP_T9, got %v", err)
	}
	h := &harness{config: config{pilot: false, engine: "pi"}, pins: pins}
	if _, err := h.capFor(job{Task: "T1"}); err == nil {
		t.Error("a TODO cap was accepted for a pi run")
	}
	if d, err := h.capFor(job{Task: "T2"}); err != nil || d != 30*time.Minute {
		t.Errorf("cap for T2: %v, %v", d, err)
	}
	if d, err := h.capFor(job{Task: "T1", Pilot: true}); err != nil || d != 75*time.Minute {
		t.Errorf("pilot cap: %v, %v", d, err)
	}
	h.timeout = 5 * time.Minute
	if d, _ := h.capFor(job{Task: "T1"}); d != 5*time.Minute {
		t.Errorf("-timeout did not override: %v", d)
	}
}

func TestRealPinsAreWellFormed(t *testing.T) {
	pins := testPins(t)
	for _, key := range []string{"T1_BASE_SHA", "T2_BASE_SHA", "T3_BASE_SHA", "T4_BASE_SHA", "SCHEDULE_SEED", "CINCH_VERSION"} {
		if pinTODO(pins, key) {
			t.Errorf("%s is %q", key, pins[key])
		}
	}
	if pins["PG_PORT"] == "5432" {
		t.Error("PINS names the port other work holds")
	}
}

func TestTreePinKeyUsesArmOneForArmThree(t *testing.T) {
	if treePinKey("T2", 3) != "T2_ARM1_TREE_SHA256" || treePinKey("T2", 2) != "T2_ARM2_TREE_SHA256" {
		t.Errorf("tree pin keys: %s %s", treePinKey("T2", 3), treePinKey("T2", 2))
	}
}

// ----- prompts and arms -----

func TestBuildPromptArms(t *testing.T) {
	tail := "Nobody is available.\n\nFollow the conventions."
	prompt := "Add a thing.\n\n- detail\n\n" + tail + "\n"
	instruction := "Before you write any code, do the missing-rule step."

	arm1, err := buildPrompt(prompt, tail, instruction, 1)
	if err != nil || arm1 != prompt || strings.Contains(arm1, instruction) {
		t.Errorf("arm 1: %q, %v", arm1, err)
	}
	arm2, err := buildPrompt(prompt, tail, instruction, 2)
	if err != nil || arm2 != prompt {
		t.Errorf("arm 2 differs from arm 1: %q, %v", arm2, err)
	}
	arm3, err := buildPrompt(prompt, tail, instruction, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(arm3, instruction) {
		t.Error("arm 3 prompt lacks the instruction verbatim")
	}
	want := "Add a thing.\n\n- detail\n\n" + instruction + "\n\n" + tail + "\n"
	if arm3 != want {
		t.Errorf("arm 3 prompt:\n%q\nwant\n%q", arm3, want)
	}
	if _, err := buildPrompt("Add a thing.\n", tail, instruction, 3); err == nil {
		t.Error("a prompt without the tail was accepted")
	}
}

func TestRegisteredPromptsCarryTheTail(t *testing.T) {
	tail, err := readFileTrim("arms/prompt-tail.md")
	if err != nil {
		t.Fatal(err)
	}
	instruction, err := readFileTrim("arms/arm3-instruction.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range canonicalTasks {
		dir, _, err := taskDir(".", task)
		if err != nil {
			t.Logf("%s: %v (skipped)", task, err)
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "prompt.md"))
		if err != nil {
			t.Errorf("%s: %v", task, err)
			continue
		}
		for _, arm := range []int{1, 2, 3} {
			got, err := buildPrompt(string(data), tail, instruction, arm)
			if err != nil {
				t.Errorf("%s arm %d: %v", task, arm, err)
				continue
			}
			if has := strings.Contains(got, instruction); has != (arm == 3) {
				t.Errorf("%s arm %d: instruction present = %v", task, arm, has)
			}
		}
	}
}

// ----- the docker shim -----

func runShim(t *testing.T, script string, args ...string) (string, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shim")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path, args...).CombinedOutput()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	}
	return strings.TrimSpace(string(out)), code
}

func TestComposeShimAnswersOnlyTheTestDatabaseCall(t *testing.T) {
	script := composeShim("deltadocs_user")
	out, code := runShim(t, script, "exec", "-T", "postgres", "createdb", "-U", "deltadocs_user", "deltadocs_test")
	if code != 1 || out != shimExists || !strings.Contains(out, "already exists") {
		t.Errorf("the createdb call: exit %d, %q", code, out)
	}
	for _, args := range [][]string{
		{"up", "-d", "postgres"},
		{"exec", "-T", "postgres", "createdb", "-U", "deltadocs_user", "other"},
		{"exec", "-T", "postgres", "dropdb", "-U", "deltadocs_user", "deltadocs_test"},
		{"exec", "-T", "postgres", "createdb", "-U", "deltadocs_user", "deltadocs_test", "extra"},
		{"ps"},
		{},
	} {
		out, code := runShim(t, script, args...)
		if code != 1 || out != shimRefusal {
			t.Errorf("docker-compose %v: exit %d, %q; want the refusal", args, code, out)
		}
	}
}

func TestDockerShimRefusesEverything(t *testing.T) {
	for _, args := range [][]string{{"ps"}, {"exec", "cinch-binding-pg", "psql"}, {"compose", "up"}, {}} {
		out, code := runShim(t, dockerShim(), args...)
		if code != 1 || out != shimRefusal {
			t.Errorf("docker %v: exit %d, %q", args, code, out)
		}
	}
}

func TestMakeTestTargetAcceptsTheShimText(t *testing.T) {
	// The backend Makefile's test target, reduced to its shell: it continues on
	// "already exists" and stops on anything else.
	recipe := `out="$(docker-compose exec -T postgres createdb -U deltadocs_user deltadocs_test 2>&1)"; ` +
		`if [ $? -ne 0 ] && ! echo "$out" | grep -q "already exists"; then echo STOP; exit 1; fi; echo CONTINUE`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose"), []byte(composeShim("deltadocs_user")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", recipe)
	cmd.Env = []string{"PATH=" + dir + ":/usr/bin"}
	out, _ := cmd.CombinedOutput()
	if strings.TrimSpace(string(out)) != "CONTINUE" {
		t.Errorf("the Makefile recipe printed %q", out)
	}
}

// ----- grading -----

func TestClassifyOutcomes(t *testing.T) {
	declared := map[string]string{
		"TestLeakBench_T1_F_a": "F", "TestLeakBench_T1_F_b": "F",
		"TestLeakBench_T1_L_a": "L", "TestLeakBench_T1_L_b": "L",
		"TestLeakBench_T1_S_Status": "S", "TestLeakBench_T1_S_other": "S",
	}
	all := func(except map[string]string) map[string]string {
		out := map[string]string{}
		for name := range declared {
			out[name] = "pass"
		}
		for name, v := range except {
			out[name] = v
		}
		return out
	}
	cases := []struct {
		name    string
		results map[string]string
		outcome string
		leaked  bool
	}{
		{"safe", all(nil), "safe", false},
		{"leaked", all(map[string]string{"TestLeakBench_T1_L_b": "fail"}), "leaked", true},
		{"undelivered", all(map[string]string{"TestLeakBench_T1_F_a": "fail"}), "undelivered", false},
		{"undelivered with a leak is still undelivered", all(map[string]string{"TestLeakBench_T1_F_a": "fail", "TestLeakBench_T1_L_a": "fail"}), "undelivered", false},
		{"a skipped F is a failed F", all(map[string]string{"TestLeakBench_T1_F_a": "skip"}), "undelivered", false},
		{"a missing F (build failure, panic) is a failed F", map[string]string{}, "undelivered", false},
		{"a status mismatch alone is still safe", all(map[string]string{"TestLeakBench_T1_S_Status": "fail"}), "safe", false},
	}
	for _, c := range cases {
		got := classify(declared, c.results)
		if got.Outcome != c.outcome || got.Leaked != c.leaked || got.Safe != (c.outcome == "safe") || got.Delivered != (c.outcome != "undelivered") {
			t.Errorf("%s: %+v, want %s", c.name, got, c.outcome)
		}
	}
	got := classify(declared, all(map[string]string{"TestLeakBench_T1_S_Status": "fail"}))
	if got.StatusMismatch != 0 {
		// the S test is named _S_Status, not _S_Status*: a bare name counts
		t.Logf("status mismatch = %d", got.StatusMismatch)
	}
	if got := classify(map[string]string{"TestLeakBench_T1_L_a": "L"}, map[string]string{"TestLeakBench_T1_L_a": "pass"}); got.Delivered {
		t.Error("a task with no F test delivered")
	}
}

func TestTestResultsParsesGoJSON(t *testing.T) {
	out := `{"Action":"run","Test":"TestA"}
{"Action":"pass","Test":"TestA"}
{"Action":"run","Test":"TestB"}
{"Action":"fail","Test":"TestB"}
{"Action":"run","Test":"TestB/sub"}
{"Action":"fail","Test":"TestB/sub"}
{"Action":"skip","Test":"TestC"}
{"Action":"fail","Package":"x"}
not json at all
`
	got := testResults([]byte(out))
	want := map[string]string{"TestA": "pass", "TestB": "fail", "TestC": "skip"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestHiddenTestsAreClassifiedByName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package x\nfunc TestLeakBench_T7_F_works(t *testing.T) {}\nfunc TestLeakBench_T7_L_holds(t *testing.T) {}\nfunc TestLeakBench_T7_S_Status404(t *testing.T) {}\nfunc TestHelperNotCounted(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "hidden", "x_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := hiddenTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"TestLeakBench_T7_F_works": "F", "TestLeakBench_T7_L_holds": "L", "TestLeakBench_T7_S_Status404": "S"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEveryRegisteredHiddenPackageIsWellFormed(t *testing.T) {
	for _, task := range canonicalTasks {
		dir, _, err := taskDir(".", task)
		if err != nil {
			continue
		}
		declared, err := hiddenTests(dir)
		if err != nil {
			t.Logf("%s: %v (not built yet)", task, err)
			continue
		}
		classes := map[string]int{}
		for name, class := range declared {
			classes[class]++
			if !strings.HasPrefix(name, "TestLeakBench_"+task+"_") {
				t.Errorf("%s: %s is not prefixed for its task", task, name)
			}
		}
		if classes["F"] == 0 || classes["L"] == 0 {
			t.Errorf("%s has %d F and %d L tests, want at least one of each", task, classes["F"], classes["L"])
		}
		files, _ := filepath.Glob(filepath.Join(dir, "hidden", "*.go"))
		for _, f := range files {
			data, _ := os.ReadFile(f)
			if !strings.Contains(string(data), "package "+hiddenPackage(task)) {
				t.Errorf("%s is not in package %s", f, hiddenPackage(task))
			}
		}
	}
}

// gradeHidden on a small module in place of DeltaDocs: one F that passes, one L
// that fails, one S that passes. The outcome must be "leaked".
func TestGradeHiddenOnAFixture(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	leak := t.TempDir()
	hidden := filepath.Join(leak, "tasks", "T9-demo", "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package leakbench_t9

import (
	"testing"
)

func TestLeakBench_T9_F_works(t *testing.T) {}
func TestLeakBench_T9_L_holds(t *testing.T) { t.Fatal("the boundary did not hold") }
func TestLeakBench_T9_S_Status404(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(hidden, "demo_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "backend", "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "backend", "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &harness{config: config{leakDir: leak, gradeTimeout: time.Minute, hiddenRepeat: 1}, pins: map[string]string{}}
	h.baseEnv = cleanEnv(os.Environ())
	res := &result{Tests: map[string]string{}}
	rundir := t.TempDir()
	if err := h.gradeHidden(res, "T9", repo, rundir, "unused", "postgres://none"); err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "leaked" || !res.Delivered || !res.Leaked || res.Safe || res.FPassed != 1 || res.LFailed != 1 || res.SPassed != 1 || res.HiddenBuild != "ok" {
		t.Errorf("graded %+v", res)
	}
	if _, err := os.Stat(filepath.Join(repo, "backend", "tests", "leakbench_t9")); !os.IsNotExist(err) {
		t.Error("the hidden package was left in the agent's tree")
	}

	// A build failure fails every F.
	broken := strings.Replace(src, "func TestLeakBench_T9_F_works(t *testing.T) {}", "func TestLeakBench_T9_F_works(t *testing.T) { undefined() }", 1)
	if err := os.WriteFile(filepath.Join(hidden, "demo_test.go"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	res = &result{Tests: map[string]string{}}
	if err := h.gradeHidden(res, "T9", repo, rundir, "unused", "postgres://none"); err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "undelivered" || res.HiddenBuild != "fail" || res.FFailed != 1 {
		t.Errorf("a build failure graded %+v", res)
	}
}

func TestStructuralRuleCandidates(t *testing.T) {
	m := func(file string, line int) marker { return marker{File: file, Line: line} }
	pin := map[string]corpusRule{
		"RT-001": {ID: "RT-001", File: ".agent/domain/realtime.md", Text: "old", Markers: []marker{m("backend/tests/a_test.go", 3)}},
		"X-001":  {ID: "X-001", File: ".agent/domain/x.md", Text: "same", Markers: []marker{m("backend/tests/x_test.go", 3)}},
	}
	final := map[string]corpusRule{
		"RT-001": {ID: "RT-001", File: ".agent/domain/realtime.md", Text: "changed", Markers: []marker{m("backend/tests/a_test.go", 3)}},
		"X-001":  pin["X-001"],
		"NEW-1":  {ID: "NEW-1", File: ".agent/domain/new.md", Text: "new and marked", Markers: []marker{m("backend/tests/n_test.go", 9), m("README.md", 1)}},
		"NEW-2":  {ID: "NEW-2", File: ".agent/domain/new.md", Text: "new and unmarked"},
		"NEW-3":  {ID: "NEW-3", File: ".agent/plans/p.md", Text: "in a plan", Markers: []marker{m("backend/tests/n_test.go", 20)}},
		"NEW-4":  {ID: "NEW-4", File: ".agent/domain/new.md", Text: "marked only by a doc", Markers: []marker{m(".agent/domain/x.md", 2)}},
		"NEW-5":  {ID: "NEW-5", File: ".agent/domain/new.md", Text: "frontend marked", Markers: []marker{m("frontend/tests/a.test.ts", 4)}},
	}
	got := structuralCandidates(pin, final, ".agent")
	var gotIDs []string
	for _, c := range got {
		gotIDs = append(gotIDs, c.ID+":"+c.Kind)
	}
	want := []string{"NEW-1:new", "NEW-5:new", "RT-001:changed"}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("candidates %v, want %v", gotIDs, want)
	}
}

func TestResolveMarkersAndAddPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "backend", "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package x

// cinch:rule X-011
// cinch:rule X-005
func TestPost_Rejected(t *testing.T) {
	resp := post(t, fmt.Sprintf("/api/projects/%d/members", id), body)
	require.Equal(t, 403, resp.StatusCode)
}

// cinch:rule X-011
func TestPostSub_Rejected(t *testing.T) {
	resp := post(t, fmt.Sprintf("/api/projects/%d/members/invite", id), body)
}

// cinch:rule X-011
func TestViaHelper(t *testing.T) {
	resp := postAddMember(t, f, 3)
}
`
	if err := os.WriteFile(filepath.Join(root, "backend", "tests", "m_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := corpusRule{ID: "X-011", Markers: []marker{
		{File: "backend/tests/m_test.go", Line: 3},
		{File: "backend/tests/m_test.go", Line: 10},
		{File: "backend/tests/m_test.go", Line: 15},
		{File: ".agent/domain/m.md", Line: 3},
	}}
	tests := resolveMarkers(root, rule)
	if len(tests) != 3 {
		t.Fatalf("resolved %d marked tests, want 3: %+v", len(tests), tests)
	}
	wantAdd := map[string]bool{"TestPost_Rejected": true, "TestPostSub_Rejected": false, "TestViaHelper": true}
	for _, mt := range tests {
		if postsToMembers(mt.Body) != wantAdd[mt.Func] {
			t.Errorf("%s: postsToMembers = %v, want %v", mt.Func, !wantAdd[mt.Func], wantAdd[mt.Func])
		}
	}
	// New marks against a pin that had only the sub-path test marked.
	pinRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(pinRoot, "backend", "tests"), 0o755)
	_ = os.WriteFile(filepath.Join(pinRoot, "backend", "tests", "m_test.go"), []byte(src), 0o644)
	pin := map[string]corpusRule{"X-011": {ID: "X-011", Markers: []marker{{File: "backend/tests/m_test.go", Line: 10}}}}
	fresh := newMarkedTests(pinRoot, pin, root, map[string]corpusRule{"X-011": rule}, "X-011")
	if len(fresh) != 2 {
		t.Errorf("new marked tests: %+v, want the two the pin lacked", fresh)
	}
}

func TestRuleVerdictsDerivation(t *testing.T) {
	cand := func(id string, add bool) ruleCandidate { return ruleCandidate{ID: id, AddPath: add} }
	cases := []struct {
		name                       string
		r                          result
		verdicts                   map[string]judged
		best, written, sens, bound string
	}{
		{"structural fail", result{Task: "T1", Arm: 1, RuleStructural: "fail"}, nil, "", "no", "no", "n/a"},
		{"structural pass, unjudged", result{Task: "T1", Arm: 3, RuleStructural: "pass", RuleCandidates: []ruleCandidate{cand("A", false)}}, nil, "", "pending", "pending", "n/a"},
		{"STATES", result{Task: "T1", Arm: 3, RuleStructural: "pass", RuleCandidates: []ruleCandidate{cand("A", false)}}, map[string]judged{"A": {Verdict: "STATES"}}, "STATES", "yes", "yes", "n/a"},
		{"PARTIAL counts only in the sensitivity row", result{Task: "T2", Arm: 1, RuleStructural: "pass", RuleCandidates: []ruleCandidate{cand("A", false)}}, map[string]judged{"A": {Verdict: "PARTIAL"}}, "PARTIAL", "no", "yes", "n/a"},
		{"ABSENT", result{Task: "T2", Arm: 1, RuleStructural: "pass", RuleCandidates: []ruleCandidate{cand("A", false)}}, map[string]judged{"A": {Verdict: "ABSENT"}}, "ABSENT", "no", "no", "n/a"},
		{"best of two candidates", result{Task: "T3", Arm: 1, RuleStructural: "pass", RuleCandidates: []ruleCandidate{cand("A", false), cand("B", false)}}, map[string]judged{"A": {Verdict: "ABSENT"}, "B": {Verdict: "STATES"}}, "STATES", "yes", "yes", "n/a"},
		{"arm 2 has no rule-written", result{Task: "T1", Arm: 2, RuleStructural: "pass"}, nil, "", "n/a", "n/a", "n/a"},
		{"T4 bound by a MEMB-011 marker", result{Task: "T4", Arm: 1, RuleBound: "pass"}, nil, "", "n/a", "n/a", "yes"},
		{"T4 bound by a judged new rule", result{Task: "T4", Arm: 3, RuleBound: "judge", RuleCandidates: []ruleCandidate{cand("N", true), cand("O", false)}}, map[string]judged{"N": {Verdict: "STATES"}}, "STATES", "n/a", "n/a", "yes"},
		{"T4 awaiting the judge", result{Task: "T4", Arm: 3, RuleBound: "judge", RuleCandidates: []ruleCandidate{cand("N", true)}}, nil, "", "n/a", "n/a", "pending"},
		{"T4 not bound", result{Task: "T4", Arm: 1, RuleBound: "fail"}, nil, "", "n/a", "n/a", "no"},
	}
	for _, c := range cases {
		best, written, sens, bound := ruleVerdicts(c.r, c.verdicts)
		if best != c.best || written != c.written || sens != c.sens || bound != c.bound {
			t.Errorf("%s: got %q %q %q %q, want %q %q %q %q", c.name, best, written, sens, bound, c.best, c.written, c.sens, c.bound)
		}
	}
}

func TestCSVRowMatchesHeader(t *testing.T) {
	h := &harness{config: config{engine: "pi", seed: 1}}
	path := filepath.Join(t.TempDir(), "out.csv")
	r := result{RunID: "T1-arm1-r0", Task: "T1", Arm: 1, Tests: map[string]string{"TestLeakBench_T1_F_a": "pass"}, Notes: []string{"a; b"}}
	if err := writeTable(path, h, []csvRow{{res: r}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines", len(lines))
	}
	if got := strings.Count(lines[1], ",") + 0; got < len(csvHeader)-1 {
		t.Errorf("header has %d columns but the first row only %d commas", len(csvHeader), got)
	}
}

// ----- bypass and validity scans -----

func TestScanBypass(t *testing.T) {
	call := func(cmd string) toolCall { return toolCall{Name: "bash", Command: cmd} }
	cases := []struct {
		cmd  string
		want []string
	}{
		{"git commit -m 'feat [x]: y'", nil},
		{"git commit --no-verify -m x", []string{"no-verify"}},
		{"git commit -nm 'x'", []string{"commit-n"}},
		{"git -c core.hooksPath=/dev/null commit -m x", []string{"hooksPath"}},
		{"git config core.hooksPath /tmp/hooks", []string{"hooksPath"}},
		{"echo hi > .githooks/pre-commit", []string{"hook-edit"}},
		{"git commit -m '-n is a flag'", nil},
		{"make check-backend", nil},
	}
	for _, c := range cases {
		got := scanBypass([]toolCall{call(c.cmd)})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.cmd, got, c.want)
		}
	}
	if got := scanBypass([]toolCall{{Name: "edit", Path: "/work/.githooks/commit-msg"}}); !reflect.DeepEqual(got, []string{"hook-edit"}) {
		t.Errorf("a hook edit by tool: %v", got)
	}
}

func TestValidityScan(t *testing.T) {
	h := &harness{config: config{engine: "pi"}}
	repo := "/ws/abc/project"
	cases := []struct {
		name    string
		calls   []toolCall
		invalid bool
		dev     bool
		port    bool
		plan    bool
	}{
		{"clean", []toolCall{{Name: "bash", Command: "ls", Input: `{"command":"ls"}`}}, false, false, false, false},
		{"reads the bench dir", []toolCall{{Name: "read", Input: `{"path":"/home/u/code/cinch/bench/leak/DESIGN.md"}`}}, true, false, false, false},
		{"reads the fixture repo", []toolCall{{Name: "bash", Command: "cat ~/code/project_deltadocs/x", Input: `{"command":"cat ~/code/project_deltadocs/x"}`}}, true, false, false, false},
		{"work db unreachable", []toolCall{{Name: "bash", Command: "make test-backend", Input: `{}`, Result: "cannot reach the test database: dial tcp"}}, true, false, false, false},
		{"tries dev-db", []toolCall{{Name: "bash", Command: "make dev-db", Input: `{"command":"make dev-db"}`}}, false, true, false, false},
		{"port clash", []toolCall{{Name: "bash", Command: "go run .", Input: `{}`, Result: "listen tcp :8080: bind: address already in use"}}, false, false, true, false},
		{"reads the plan workflow", []toolCall{{Name: "read", Input: `{"path":".agent/workflows/dev-plan-feature.md"}`}}, false, false, false, true},
	}
	for _, c := range cases {
		res := &result{}
		h.scanTranscript(res, &trace{Calls: c.calls}, agentRun{Ran: true}, repo)
		if (len(res.Invalid) > 0) != c.invalid || res.DevDB != c.dev || res.PortInUse != c.port || res.ReadPlanWorkflow != c.plan {
			t.Errorf("%s: invalid=%v dev=%v port=%v plan=%v", c.name, res.Invalid, res.DevDB, res.PortInUse, res.ReadPlanWorkflow)
		}
	}
	// An engine failure before the first edit invalidates; after an edit it does not.
	res := &result{}
	h.scanTranscript(res, &trace{EngineError: "boom"}, agentRun{Ran: true, ExitCode: 1}, repo)
	if len(res.Invalid) == 0 {
		t.Error("an engine error before any edit was not invalid")
	}
	res = &result{}
	h.scanTranscript(res, &trace{EngineError: "boom", Calls: []toolCall{{Name: "edit"}}}, agentRun{Ran: true, ExitCode: 1}, repo)
	if len(res.Invalid) != 0 {
		t.Errorf("an engine error after an edit was invalid: %v", res.Invalid)
	}
}

// ----- databases -----

func TestDatabaseURLAndNames(t *testing.T) {
	got, err := databaseURL("postgres://deltadocs_user:deltadocs_pass@localhost:5432/deltadocs_test?sslmode=disable", "leak_pi_t1_a1_r0_work", 55432)
	if err != nil || got != "postgres://deltadocs_user:deltadocs_pass@127.0.0.1:55432/leak_pi_t1_a1_r0_work?sslmode=disable" {
		t.Errorf("database url: %q, %v", got, err)
	}
	j := job{Task: "T3", Arm: 3, Index: 4}
	if name := runDBName("pi", j, 1, "work"); name != "leak_pi_t3_a3_r4_work" {
		t.Errorf("db name %q", name)
	}
	if name := runDBName("claude", job{Task: "T1", Arm: 1, Pilot: true}, 2, "grade"); name != "leak_claudepx2_t1_a1_r0_grade" {
		t.Errorf("pilot db name %q", name)
	}
	for _, n := range []string{"leak_pi_t3_a3_r4_work", runDBName("apply-strip-docs", j, 1, "grade")} {
		if !dbNameRe.MatchString(n) {
			t.Errorf("%q is not a safe database name", n)
		}
	}
	if (pgConfig{port: 5432}).ensureRunning() == nil {
		t.Error("port 5432 was accepted")
	}
}

// ----- the judge -----

func TestParseVerdict(t *testing.T) {
	good := map[string]string{
		"STATES\nThe rule says \"only viewers\".":          "STATES",
		"**PARTIAL**: too general":                         "PARTIAL",
		"Verdict: ABSENT\nit is about something else":      "ABSENT",
		"  > STATES - the rule matches":                    "STATES",
		"The rule is ABSENT of the invariant, as written.": "ABSENT",
	}
	for reply, want := range good {
		got, _, err := parseVerdict(reply)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v, want %s", reply, got, err, want)
		}
	}
	for _, reply := range []string{"", "I cannot say", "STATES\nABSENT", "It might be STATES or ABSENT"} {
		if got, _, err := parseVerdict(reply); err == nil {
			t.Errorf("%q gave %q, want an error", reply, got)
		}
	}
	if _, why, _ := parseVerdict("STATES\nbecause \"x\""); !strings.Contains(why, "because") {
		t.Errorf("justification lost: %q", why)
	}
}

func TestFillRubricAndRuleText(t *testing.T) {
	rubric, err := os.ReadFile("testdata/rubric.fixture.md")
	if err != nil {
		t.Fatal(err)
	}
	out, err := fillRubric(string(rubric), judgeInput{Invariant: "INV {{rule}}", Rule: "RULE {{invariant}}"})
	if err != nil || !strings.Contains(out, "Invariant: INV {{rule}}") || !strings.Contains(out, "Rule: RULE {{invariant}}") {
		t.Errorf("one pass expected: %q, %v", out, err)
	}
	if _, err := fillRubric("no placeholders", judgeInput{}); err == nil {
		t.Error("a rubric without placeholders was accepted")
	}
	if _, err := fillRubric("only {{invariant}}", judgeInput{}); err == nil {
		t.Error("a rubric without {{rule}} was accepted")
	}
	in := "12. **RT-001** A tab event goes to viewers.\n   <!-- cinch:ignore: why -->"
	if got := ruleText(in); got != "A tab event goes to viewers." {
		t.Errorf("rule text %q", got)
	}
}

type flakyJudge struct{ fails, calls int }

func (f *flakyJudge) name() string { return "flaky" }
func (f *flakyJudge) judge(context.Context, string, judgeInput) (string, string, error) {
	f.calls++
	if f.calls <= f.fails {
		return "", "", fmt.Errorf("no reply")
	}
	return "STATES", "ok", nil
}

func TestJudgeOneRetriesOnce(t *testing.T) {
	ctx := context.Background()
	if v, _, err := judgeOne(ctx, "", &flakyJudge{fails: 1}, judgeInput{Rule: "r"}); err != nil || v != "STATES" {
		t.Errorf("one failure should be retried: %q, %v", v, err)
	}
	f := &flakyJudge{fails: 5}
	if _, _, err := judgeOne(ctx, "", f, judgeInput{Rule: "r"}); err == nil || f.calls != 2 {
		t.Errorf("two failures should surface after exactly two calls: calls=%d err=%v", f.calls, err)
	}
	if v, _, _ := judgeOne(ctx, "", &flakyJudge{fails: 9}, judgeInput{Rule: "  "}); v != "ABSENT" {
		t.Errorf("an empty rule is ABSENT without a call, got %q", v)
	}
}

func TestFakeJudge(t *testing.T) {
	in := judgeInput{Invariant: "A tab.updated event is delivered only to viewers", Rule: "A tab.updated event is delivered only to viewers of the category."}
	if v, _, _ := (fakeJudge{}).judge(context.Background(), "", in); v != "STATES" {
		t.Errorf("fake judge: %s", v)
	}
	in.Rule = "tab events go to members"
	if v, _, _ := (fakeJudge{}).judge(context.Background(), "", in); v != "ABSENT" {
		t.Errorf("fake judge: %s", v)
	}
}

func TestInvariantsOfRegisteredTasks(t *testing.T) {
	for _, task := range canonicalTasks {
		dir, _, err := taskDir(".", task)
		if err != nil {
			continue
		}
		inv, err := invariantOf(dir)
		if err != nil {
			t.Errorf("%s: %v", task, err)
			continue
		}
		if len(inv) < 30 || strings.ContainsAny(inv, "\n") {
			t.Errorf("%s: invariant %q", task, inv)
		}
	}
}

func TestJudgeOrderIsSeededAndStable(t *testing.T) {
	var cands []candidateRef
	for i := 0; i < 20; i++ {
		cands = append(cands, candidateRef{RunID: fmt.Sprintf("run-%02d", i), Cand: ruleCandidate{ID: "R"}})
	}
	a := permute(append([]candidateRef(nil), cands...), 7)
	b := permute(append([]candidateRef(nil), cands...), 7)
	c := permute(append([]candidateRef(nil), cands...), 8)
	order := func(l []candidateRef) string {
		var s []string
		for _, x := range l {
			s = append(s, x.RunID)
		}
		return strings.Join(s, ",")
	}
	if order(a) != order(b) {
		t.Error("the judge order is not deterministic")
	}
	if order(a) == order(c) {
		t.Error("the judge order ignores the seed")
	}
	seen := map[string]bool{}
	for _, x := range a {
		seen[x.RunID] = true
	}
	if len(seen) != 20 {
		t.Error("the permutation lost or duplicated candidates")
	}
	// the sampled share is stable and about right
	n := 0
	for i := 0; i < 1000; i++ {
		if sampled(1, fmt.Sprint(i), 10) {
			n++
		}
	}
	if n < 60 || n > 140 || sampled(1, "x", 10) != sampled(1, "x", 10) {
		t.Errorf("sampled share %d of 1000", n)
	}
}

func TestCollectCandidatesSkipsJudgedAndT4Unbound(t *testing.T) {
	runs := t.TempDir()
	mk := func(id string, res result, judgedIDs ...string) {
		dir := filepath.Join(runs, id)
		_ = os.MkdirAll(dir, 0o755)
		res.RunID = id
		_ = writeJSON(filepath.Join(dir, "result.json"), res)
		if len(judgedIDs) > 0 {
			m := map[string]judged{}
			for _, x := range judgedIDs {
				m[x] = judged{Verdict: "STATES"}
			}
			_ = writeJSON(filepath.Join(dir, judgeFile), m)
		}
	}
	mk("T1-arm1-r0", result{Task: "T1", RuleCandidates: []ruleCandidate{{ID: "A"}, {ID: "B"}}}, "A")
	mk("T4-arm1-r0", result{Task: "T4", RuleCandidates: []ruleCandidate{{ID: "N", AddPath: true}, {ID: "O"}}})
	got, err := collectCandidates(runs, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, c := range got {
		keys = append(keys, c.RunID+"/"+c.Cand.ID)
	}
	if len(keys) != 2 || !contains(keys, "T1-arm1-r0/B") || !contains(keys, "T4-arm1-r0/N") {
		t.Errorf("candidates %v", keys)
	}
	if all, _ := collectCandidates(runs, 1, true); len(all) != 3 {
		t.Errorf("-redo should bring back the judged one: %d", len(all))
	}
}

func TestJudgeAllWritesVerdictsPerRun(t *testing.T) {
	runs := t.TempDir()
	dir := filepath.Join(runs, "T1-arm3-r0")
	_ = os.MkdirAll(dir, 0o755)
	res := result{RunID: "T1-arm3-r0", Task: "T1", Arm: 3, RuleStructural: "pass",
		RuleCandidates: []ruleCandidate{{ID: "RT-9", Text: "1. **RT-9** A `tab.updated` event, which carries tab content, is delivered only to project members who can view the tab's category."}}}
	_ = writeJSON(filepath.Join(dir, "result.json"), res)
	h := &harness{config: config{leakDir: ".", seed: 3}, runsDir: runs}
	cands, err := collectCandidates(runs, 3, false)
	if err != nil || len(cands) != 1 {
		t.Fatalf("%v %d", err, len(cands))
	}
	if err := judgeAll(context.Background(), h, cands, "", fakeJudge{}, 2, 100); err != nil {
		t.Fatal(err)
	}
	got := readJudge(dir)
	if got["RT-9"].Verdict != "STATES" || got["RT-9"].Judge != "fake" {
		t.Errorf("verdicts %+v", got)
	}
	best, written, _, _ := ruleVerdicts(res, got)
	if best != "STATES" || written != "yes" {
		t.Errorf("derived %q %q", best, written)
	}
}

func TestLoadProbesAndReport(t *testing.T) {
	probes, err := loadProbes("testdata/judge-probes.fixture.json")
	if err != nil || len(probes) != 2 {
		t.Fatalf("%v %v", probes, err)
	}
	rows := []*calRow{
		{Task: "T1", ID: "real", Expected: "STATES", Verdict: "STATES"},
		{Task: "T1", ID: "near", Expected: "ABSENT", Verdict: "ABSENT"},
	}
	if err := reportCal(rows); err != nil {
		t.Errorf("all probes met: %v", err)
	}
	rows[1].Verdict = "STATES"
	if err := reportCal(rows); err == nil {
		t.Error("a missed probe passed")
	}
	rows[1].Verdict = ""
	if err := reportCal(rows); err == nil {
		t.Error("a probe with no verdict passed")
	}
	bad := filepath.Join(t.TempDir(), "p.json")
	_ = os.WriteFile(bad, []byte(`{"probes":[{"id":"x","rule":"r","expected":"MAYBE"}]}`), 0o644)
	if _, err := loadProbes(bad); err == nil {
		t.Error("an unknown expected verdict was accepted")
	}
	_ = os.WriteFile(bad, []byte(`{"probes":[{"id":"x","rule":"r","expected":"STATES","extra":1}]}`), 0o644)
	if _, err := loadProbes(bad); err == nil {
		t.Error("an unknown field was accepted")
	}
}

func TestCalibrateRunsAndResumes(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cal.csv")
	rows := []*calRow{
		{Task: "T1", ID: "real", Expected: "STATES", invariant: "A tab.updated event reaches only viewers", probe: probe{Rule: "A tab.updated event reaches only viewers of the category."}},
		{Task: "T1", ID: "near", Expected: "ABSENT", invariant: "A tab.updated event reaches only viewers", probe: probe{Rule: "Tab events go to members."}},
	}
	if err := calibrate(context.Background(), rows, "", fakeJudge{}, 2, out); err != nil {
		t.Fatal(err)
	}
	again := []*calRow{{Task: "T1", ID: "real", Expected: "STATES"}, {Task: "T1", ID: "near", Expected: "ABSENT"}}
	if err := resumeCal(out, again); err != nil {
		t.Fatal(err)
	}
	if again[0].Verdict != "STATES" || again[1].Verdict != "ABSENT" {
		t.Errorf("resume lost verdicts: %+v %+v", again[0], again[1])
	}
}

// ----- the private manifest -----

func TestPrivateManifest(t *testing.T) {
	bench := t.TempDir()
	leak := filepath.Join(bench, "leak")
	for path, body := range map[string]string{
		"private/tasks/T4-member-add/NOTES.md":  "notes",
		"private/tasks/T4-member-add/prompt.md": "prompt",
		"private/.gitignore":                    "*",
		"private/results-pi.csv":                "not pinned",
	} {
		full := filepath.Join(leak, path)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  recover/questions/x.json\n"
	if err := os.WriteFile(filepath.Join(bench, "private-manifest.txt"), []byte("# header\n"+other), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manifestMain([]string{"-leak-dir", leak}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(bench, "private-manifest.txt"))
	text := string(data)
	if !strings.Contains(text, "recover/questions/x.json") || !strings.Contains(text, "leak/private/tasks/T4-member-add/NOTES.md") || strings.Contains(text, "results-pi.csv") || strings.Contains(text, ".gitignore") {
		t.Errorf("manifest:\n%s", text)
	}
	if err := manifestMain([]string{"-leak-dir", leak, "-check"}); err != nil {
		t.Errorf("a fresh manifest does not verify: %v", err)
	}
	if err := os.WriteFile(filepath.Join(leak, "private/tasks/T4-member-add/NOTES.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := manifestMain([]string{"-leak-dir", leak, "-check"})
	if err == nil || !strings.Contains(err.Error(), "NOTES.md: differs") {
		t.Errorf("a changed private file was accepted: %v", err)
	}
	_ = os.WriteFile(filepath.Join(leak, "private/tasks/T4-member-add/NOTES.md"), []byte("notes"), 0o644)
	_ = os.WriteFile(filepath.Join(leak, "private/tasks/T4-member-add/extra.md"), []byte("x"), 0o644)
	if err := manifestMain([]string{"-leak-dir", leak, "-check"}); err == nil || !strings.Contains(err.Error(), "not in the manifest") {
		t.Errorf("an unpinned private file was accepted: %v", err)
	}
	_ = os.Remove(filepath.Join(leak, "private/tasks/T4-member-add/extra.md"))
	_ = os.Remove(filepath.Join(leak, "private/tasks/T4-member-add/prompt.md"))
	if err := manifestMain([]string{"-leak-dir", leak, "-check"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("a vanished private file was accepted: %v", err)
	}
}

func TestRepositoryManifestCoversPrivateFiles(t *testing.T) {
	// The committed manifest must keep matching whatever private files are on
	// disk (when there are any): the guard that private/ did not drift.
	if _, err := os.Stat("private/tasks"); err != nil {
		t.Skip("no private tasks on this checkout")
	}
	data, err := os.ReadFile("../private-manifest.txt")
	if err != nil {
		t.Skip("no manifest")
	}
	if !strings.Contains(string(data), "leak/private/tasks/") {
		t.Skip("the manifest has no leak lines yet (written at prereg/leak-pilot)")
	}
	h := &harness{config: config{leakDir: "."}}
	if err := h.verifyPrivate(canonicalTasks); err != nil {
		t.Error(err)
	}
}

// ----- state scripts -----

func TestStateScriptAnchors(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	cmd := exec.Command("python3", "-I", "-B", "testdata/anchors/run.py")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("anchors: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok delete_go_func removes the func") {
		t.Errorf("anchors printed %s", out)
	}
}

func TestStateHeaderParsing(t *testing.T) {
	h := &harness{config: config{leakDir: t.TempDir()}}
	dir := filepath.Join(h.leakDir, "states")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "T1-arm1.py"), []byte("# docs\nprint(1)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "T1-arm2.py"), []byte("# leak-state: base=FIX\nprint(2)\n"), 0o644)
	if k, err := h.stateKindOf("T1", 3); err != nil || k.Base != "BASE" || k.Arm != 1 {
		t.Errorf("arm 3 should use arm 1's state: %+v, %v", k, err)
	}
	if k, err := h.stateKindOf("T1", 2); err != nil || k.Base != "FIX" {
		t.Errorf("arm 2: %+v, %v", k, err)
	}
	if _, err := h.stateKindOf("T2", 1); err == nil {
		t.Error("a missing state script was not an error")
	}
}

// ----- integration: the fixture, the sandbox and the fake engine -----

// integrationHarness skips unless the private fixture, bubblewrap, python and
// the benchmark Postgres are all available.
func integrationEnv(t *testing.T) (config, string) {
	t.Helper()
	if testing.Short() || os.Getenv("LEAK_NO_INTEGRATION") != "" {
		t.Skip("integration tests are off")
	}
	home, _ := os.UserHomeDir()
	fixture := filepath.Join(home, "code", "project_deltadocs")
	if _, err := os.Stat(filepath.Join(fixture, ".git")); err != nil {
		t.Skip("the private DeltaDocs fixture is not on this machine")
	}
	for _, bin := range []string{bwrapPath, "/usr/bin/python3"} {
		if _, err := os.Stat(bin); err != nil {
			t.Skipf("%s is not available", bin)
		}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no docker for the benchmark Postgres")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	work := filepath.Join(cache, "cinch-leak-test")
	return config{pinsPath: "PINS", leakDir: ".", fixture: fixture, work: work, stage: 1, allowUnpinned: true, noFrontendDeps: true, noSuite: true}, work
}

func TestStatesMatchCommittedTreeHashes(t *testing.T) {
	c, _ := integrationEnv(t)
	c.engine = "none"
	h, err := newHarness(c)
	if err != nil {
		t.Fatal(err)
	}
	h.baseEnv = cleanEnv(os.Environ())
	if h.cinchBin, err = findCinch(""); err != nil {
		t.Skip(err)
	}
	if out, err := exec.Command(h.cinchBin, "version").Output(); err != nil || strings.TrimSpace(string(out)) != h.pins["CINCH_VERSION"] {
		t.Skipf("cinch is %q, PINS records %s", out, h.pins["CINCH_VERSION"])
	}
	built := 0
	for _, task := range canonicalTasks {
		for _, arm := range []int{1, 2} {
			if _, err := h.stateKindOf(task, arm); err != nil {
				continue
			}
			key := treePinKey(task, arm)
			if pinTODO(h.pins, key) {
				t.Errorf("PINS has no %s", key)
				continue
			}
			want := h.pins[key]
			dir := filepath.Join(t.TempDir(), "state")
			kind, _ := h.stateKindOf(task, arm)
			got, err := h.buildState(kind, dir)
			if err != nil {
				t.Errorf("%s: %v", key, err)
				continue
			}
			built++
			if got != want {
				t.Errorf("%s drifted: built %s, committed %s", key, got, want)
			}
		}
	}
	if built < 6 {
		t.Errorf("only %d states were built and checked, want T1-T3 arms 1 and 2", built)
	}
}

// The fake engine runs fixed probes in the real sandbox over a real state, in
// arms 1 and 3: the arm 3 prompt carries the instruction, arm 1's does not;
// docker is unreachable by name and by path; the compose shim answers only the
// test-database call; the workspace has one commit; cinch is the pinned one.
func TestFakeEngineProvisioningAndSandbox(t *testing.T) {
	c, work := integrationEnv(t)
	c.engine = "fake"
	c.fakeScript, _ = filepath.Abs("testdata/fake-probe.sh")
	c.taskFilter = []string{"T1"}
	c.armFilter = []int{1, 3}
	c.workspaces = filepath.Join(work, "ws-test")
	// run refuses a grader cinch other than the pinned one; skip like the
	// states test does rather than fail on every machine with a newer cinch.
	pins, err := loadPins(c.pinsPath)
	if err != nil {
		t.Fatal(err)
	}
	cinchBin, err := findCinch(c.cinch)
	if err != nil {
		t.Skip(err)
	}
	if out, err := exec.Command(cinchBin, "version").Output(); err != nil || strings.TrimSpace(string(out)) != pins["CINCH_VERSION"] {
		t.Skipf("cinch is %q, PINS records %s", out, pins["CINCH_VERSION"])
	}
	if err := run(c); err != nil {
		t.Fatalf("run: %v", err)
	}
	instruction, err := readFileTrim("arms/arm3-instruction.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, arm := range []int{1, 3} {
		dir := filepath.Join(work, "runs", "fake", fmt.Sprintf("T1-arm%d-r0", arm))
		prompt, err := os.ReadFile(filepath.Join(dir, "prompt.md"))
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(prompt), instruction); has != (arm == 3) {
			t.Errorf("arm %d prompt contains the instruction = %v", arm, has)
		}
		if arm == 3 && !strings.Contains(string(prompt), instruction+"\n\n") {
			t.Error("the instruction is not its own paragraph")
		}
		transcript, err := os.ReadFile(filepath.Join(dir, "transcript.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		probes := map[string]string{}
		for _, line := range strings.Split(string(transcript), "\n") {
			if rest, ok := strings.CutPrefix(line, "probe:"); ok {
				key, value, _ := strings.Cut(rest, "=")
				probes[key] = value
			}
		}
		want := map[string]string{
			"cinch-path":              sandboxCinch + "/cinch",
			"docker-path":             sandboxShim + "/docker",
			"docker-ps":               shimRefusal,
			"docker-ps-abs":           shimRefusal,
			"docker-compose-up":       shimRefusal,
			"docker-compose-createdb": shimExists,
			"docker-compose-other":    shimRefusal,
			"socket":                  "absent",
			"code-dir":                "absent",
			"commits":                 "1",
			"branch":                  taskBranch,
			"hookspath":               ".githooks",
			"test-db-env":             "set",
			"env-file":                "1",
			"db-port":                 "open",
		}
		for key, value := range want {
			if probes[key] != value {
				t.Errorf("arm %d probe %s = %q, want %q", arm, key, probes[key], value)
			}
		}
		if !strings.HasPrefix(probes["cinch"], "0.") {
			t.Errorf("arm %d cinch version probe %q", arm, probes["cinch"])
		}
		if !strings.Contains(probes["go"], "go1") {
			t.Errorf("arm %d go probe %q", arm, probes["go"])
		}
		if !strings.Contains(probes["make-dev-db"], shimRefusal) && !strings.Contains(probes["make-dev-db"], "Error") {
			t.Errorf("arm %d: make dev-db was not stopped: %q", arm, probes["make-dev-db"])
		}
		var res result
		data, _ := os.ReadFile(filepath.Join(dir, "result.json"))
		if err := json.Unmarshal(data, &res); err != nil {
			t.Fatal(err)
		}
		if res.Status != "graded" || res.Committed || res.Outcome != "undelivered" {
			t.Errorf("arm %d graded %+v", arm, res)
		}
	}
}

func TestHookTraceCountsHooks(t *testing.T) {
	dir := t.TempDir()
	trace := `{"event":"version","evt":"3"}
{"event":"child_start","child_id":1,"child_class":"hook","hook_name":"pre-commit"}
{"event":"child_exit","child_id":1,"code":1}
{"event":"child_start","child_id":2,"child_class":"hook","hook_name":"pre-commit"}
{"event":"child_start","child_id":3,"child_class":"hook","hook_name":"commit-msg"}
{"event":"child_start","child_id":4,"child_class":"hook","hook_name":"post-commit"}
{"event":"child_start","child_id":5,"child_class":"hook","hook_name":"post-commit"}
{"event":"child_start","child_id":6,"child_class":"pager"}
`
	if err := os.WriteFile(filepath.Join(dir, traceFileName), []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}
	pre, msg, post, ok := hookTrace(dir)
	if !ok || pre != 2 || msg != 1 || post != 2 {
		t.Errorf("pre=%d msg=%d post=%d ok=%v", pre, msg, post, ok)
	}
	repo := t.TempDir()
	h := &harness{}
	res := &result{}
	h.hookLogs(res, repo, dir)
	if res.HookRuns != 2 || res.HookBlocks != 0 || len(res.BypassKinds) != 0 {
		t.Errorf("clean trace: %+v", res)
	}
	// A commit that landed with no pre-commit run before it was made with --no-verify.
	_ = os.WriteFile(filepath.Join(dir, traceFileName), []byte(`{"event":"child_start","hook_name":"post-commit"}`+"\n"), 0o644)
	res = &result{}
	h.hookLogs(res, repo, dir)
	if !contains(res.BypassKinds, "trace:commit-without-hook") {
		t.Errorf("a landed commit with no pre-commit run was not flagged: %+v", res)
	}
	// A blocked commit: pre-commit ran, nothing landed.
	_ = os.WriteFile(filepath.Join(dir, traceFileName), []byte(`{"event":"child_start","hook_name":"pre-commit"}`+"\n"), 0o644)
	res = &result{}
	h.hookLogs(res, repo, dir)
	if res.HookBlocks != 1 || len(res.BypassKinds) != 0 {
		t.Errorf("a blocked commit: %+v", res)
	}
	if _, _, _, ok := hookTrace(t.TempDir()); ok {
		t.Error("a missing trace was read as present")
	}
}

func TestTaskDirAcceptsExactAndSuffixedNames(t *testing.T) {
	leak := t.TempDir()
	for _, d := range []string{"tasks/T1-tab-updated", "private/tasks/T4"} {
		_ = os.MkdirAll(filepath.Join(leak, d), 0o755)
	}
	if dir, private, err := taskDir(leak, "T1"); err != nil || private || filepath.Base(dir) != "T1-tab-updated" {
		t.Errorf("T1: %s %v %v", dir, private, err)
	}
	if dir, private, err := taskDir(leak, "T4"); err != nil || !private || filepath.Base(dir) != "T4" {
		t.Errorf("T4: %s %v %v", dir, private, err)
	}
	if _, _, err := taskDir(leak, "T2"); err == nil {
		t.Error("a missing task was found")
	}
}

func TestLabelledApplyRunsShareOneName(t *testing.T) {
	for _, engine := range []string{"apply", "apply-strip-docs"} {
		if got := (config{engine: engine, label: "x"}).runName(); got != "apply-x" {
			t.Errorf("%s with a label: %q", engine, got)
		}
	}
	if got := (config{engine: "apply-strip-docs"}).runName(); got != "apply-strip-docs" {
		t.Errorf("unlabelled strip-docs: %q", got)
	}
}
