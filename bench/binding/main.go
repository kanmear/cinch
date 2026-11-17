// Command binding measures binding strength: for each rule a `cinch:rule`
// marker binds to a Go test, a local model proposes the smallest production
// change that violates the rule, and the full test suite says whether the
// bound test, some other test, or nothing caught it. The method and its
// grading key are pre-registered in bench/README.md § Binding strength.
//
// Usage: go run ./bench/binding [-repo PATH] [-sha SHA] [-rules ID,ID]
//
//	[-apply ID=DIFF] [-work DIR] [-o bench/binding-strength.csv]
//
// The fixture repo is only ever cloned from. -apply grades a hand-written diff
// for one rule instead of asking the model, which is how the classifier is
// checked against a known bite (T3's SIG-019 mutation) before a real run.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	_ "embed"
)

//go:embed prompt.md
var promptTemplate string

//go:embed retry.md
var retryTemplate string

type config struct {
	repo, sha, work, cinch      string
	provider, model, thinking   string
	container, pgUser, db       string
	out, mutations              string
	proposeTimeout, testTimeout time.Duration
	baselineRuns, pgPort        int
	only                        map[string]bool
	applyRule, applyDiff        string
	regrade                     bool
}

func main() {
	home, _ := os.UserHomeDir()
	var c config
	var rules, apply, regrade string
	flag.StringVar(&c.repo, "repo", filepath.Join(home, "code", "project_deltadocs"), "fixture repo, cloned from and never written to")
	flag.StringVar(&c.sha, "sha", "ec4df00", "fixture commit to measure")
	flag.StringVar(&c.work, "work", defaultWork(), "cache directory for the clone and per-rule runs")
	flag.StringVar(&c.cinch, "cinch", "cinch", "cinch binary used for the rule inventory")
	flag.StringVar(&c.provider, "provider", "llama-cpp", "pi provider")
	flag.StringVar(&c.model, "model", "Qwen3.8-27B-UD-Q4_K_M", "pi model")
	flag.StringVar(&c.thinking, "thinking", "high", "pi thinking level")
	flag.StringVar(&c.container, "container", "cinch-binding-pg", "Postgres container holding the test database")
	flag.IntVar(&c.pgPort, "pg-port", 55432, "host port of that container (0 keeps the fixture's own)")
	flag.StringVar(&c.pgUser, "pg-user", "deltadocs_user", "Postgres user in that container")
	flag.StringVar(&c.db, "db", "deltadocs_test_binding", "test database, created if missing")
	flag.StringVar(&c.out, "o", filepath.Join("bench", "binding-strength.csv"), "CSV output")
	flag.StringVar(&c.mutations, "mutations", filepath.Join("bench", "binding", "mutations"), "directory for each rule's mutation diff")
	flag.DurationVar(&c.proposeTimeout, "propose-timeout", 15*time.Minute, "wall clock per model proposal")
	flag.DurationVar(&c.testTimeout, "test-timeout", 20*time.Minute, "wall clock per test-suite run")
	flag.IntVar(&c.baselineRuns, "baseline-runs", 2, "unmutated suite runs; a test failing in any is excluded as flaky")
	flag.StringVar(&rules, "rules", "", "comma-separated rule IDs to run (default: every bound rule)")
	flag.StringVar(&apply, "apply", "", "ID=DIFF: grade this diff for rule ID instead of asking the model")
	flag.StringVar(&regrade, "regrade", "", "comma-separated rule IDs: re-run the suite on each rule's stored mutation, without asking the model again")
	flag.Parse()

	if rules != "" {
		c.only = map[string]bool{}
		for _, id := range strings.Split(rules, ",") {
			c.only[strings.TrimSpace(id)] = true
		}
	}
	if regrade != "" {
		if rules != "" || apply != "" {
			fail(fmt.Errorf("-regrade can't be combined with -rules or -apply"))
		}
		c.regrade = true
		c.only = map[string]bool{}
		for _, id := range strings.Split(regrade, ",") {
			c.only[strings.TrimSpace(id)] = true
		}
	}
	if apply != "" {
		id, path, ok := strings.Cut(apply, "=")
		if !ok {
			fail(fmt.Errorf("-apply wants ID=DIFF, got %q", apply))
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			fail(err)
		}
		c.applyRule, c.applyDiff = id, abs
		c.only = map[string]bool{id: true}
	}
	if err := run(c); err != nil {
		fail(err)
	}
}

func defaultWork() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cinch-binding")
	}
	return filepath.Join(os.TempDir(), "cinch-binding")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "binding:", err)
	os.Exit(1)
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}

// rule is one bound rule from `cinch rules --json`, with its marked tests resolved.
type rule struct {
	ID, Doc, Text string
	Markers       []string // file:line
	Tests         []boundTest
}

type boundTest struct {
	File, Name, Package string // Package is the Go import path go test -json reports
	Line                int
	Source              string // marker comments through the closing brace
}

func (b boundTest) key() string { return b.Package + "." + b.Name }

// result is what one rule's run records in meta.json and the CSV.
type result struct {
	ID           string   `json:"id"`
	Doc          string   `json:"doc"`
	Markers      []string `json:"markers"`
	BoundTests   []string `json:"bound_tests"`
	Outcome      string   `json:"outcome"` // marked, other, silent, invalid, nocompile
	Reason       string   `json:"reason,omitempty"`
	FailingTests []string `json:"failing_tests"`
	Mutation     string   `json:"mutation"`
	Files        []string `json:"files"`
	LinesChanged int      `json:"lines_changed"`
	Retried      bool     `json:"retried"`
	Applied      bool     `json:"applied,omitempty"`
	Regraded     string   `json:"regraded,omitempty"` // the outcome before a re-grade, when there was one
	ProposeMS    int64    `json:"propose_ms"`
	TestMS       int64    `json:"test_ms"`
	Usage        usage    `json:"usage"`
}

type usage struct {
	Input     int            `json:"input"`
	Output    int            `json:"output"`
	CacheRead int            `json:"cache_read"`
	Turns     int            `json:"turns"`
	TimedOut  bool           `json:"timed_out"`
	ExitCode  int            `json:"exit_code"`
	Tools     map[string]int `json:"tools"`
}

func run(c config) error {
	repo := filepath.Join(c.work, "repo")
	if err := provision(c, repo); err != nil {
		return fmt.Errorf("provision: %w", err)
	}
	testURL, err := testDatabaseURL(c)
	if err != nil {
		return err
	}
	if err := createDB(c); err != nil {
		return fmt.Errorf("test database: %w", err)
	}

	all, err := inventory(c, repo)
	if err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	var selected []rule
	for _, r := range all {
		if c.only == nil || c.only[r.ID] {
			selected = append(selected, r)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("no bound rule matches -rules")
	}
	logf("%d bound rules, running %d", len(all), len(selected))

	flaky, err := baseline(c, repo, testURL)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}

	runs := filepath.Join(c.work, "runs-"+c.sha)
	if c.applyRule != "" {
		runs = filepath.Join(c.work, "control")
		_ = os.RemoveAll(filepath.Join(runs, c.applyRule))
	}
	for i, r := range selected {
		dir := filepath.Join(runs, r.ID)
		if c.regrade {
			logf("[%d/%d] %s: re-grading the stored mutation", i+1, len(selected), r.ID)
			res, err := regradeRule(c, repo, testURL, dir, flaky)
			if err != nil {
				return fmt.Errorf("%s: %w", r.ID, err)
			}
			logf("[%d/%d] %s: %s (was %s) %s", i+1, len(selected), r.ID, res.Outcome, res.Regraded, strings.Join(res.FailingTests, " "))
			if err := writeJSON(filepath.Join(dir, "meta.json"), res); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "meta.json")); err == nil {
			logf("[%d/%d] %s: already done", i+1, len(selected), r.ID)
			continue
		}
		logf("[%d/%d] %s", i+1, len(selected), r.ID)
		res, err := measure(c, repo, testURL, dir, r, flaky)
		if err != nil {
			return fmt.Errorf("%s: %w", r.ID, err)
		}
		logf("[%d/%d] %s: %s %s", i+1, len(selected), r.ID, res.Outcome, strings.Join(res.FailingTests, " "))
		if err := writeJSON(filepath.Join(dir, "meta.json"), res); err != nil {
			return err
		}
	}
	if c.applyRule != "" {
		return nil // a control run never touches the published table
	}
	return writeOutputs(c, runs, all)
}

// provision makes repo a clone of c.repo at c.sha with a clean tree. The
// fixture is a local clone so nothing the model edits can reach it.
func provision(c config, repo string) error {
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		logf("cloning %s into %s", c.repo, repo)
		if err := os.MkdirAll(filepath.Dir(repo), 0o755); err != nil {
			return err
		}
		if out, err := exec.Command("git", "clone", "--quiet", "--local", "--no-hardlinks", c.repo, repo).CombinedOutput(); err != nil {
			return fmt.Errorf("git clone: %v: %s", err, out)
		}
	}
	if _, err := git(repo, "checkout", "--quiet", "-B", "bench", c.sha); err != nil {
		return err
	}
	return resetTree(repo)
}

func resetTree(repo string) error {
	if _, err := git(repo, "reset", "--quiet", "--hard", "HEAD"); err != nil {
		return err
	}
	_, err := git(repo, "clean", "-fdq")
	return err
}

// testDatabaseURL reuses the fixture's own TEST_DATABASE_URL with the database
// name and port swapped, so this run can't collide with the developer's test
// DB or with whatever else holds the default Postgres port.
func testDatabaseURL(c config) (string, error) {
	env, err := os.ReadFile(filepath.Join(c.repo, "backend", ".env"))
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^TEST_DATABASE_URL=(\S+)`).FindSubmatch(env)
	if m == nil {
		return "", fmt.Errorf("no TEST_DATABASE_URL in %s/backend/.env", c.repo)
	}
	url := regexp.MustCompile(`([^/])/[^/?]+(\?|$)`).ReplaceAllString(string(m[1]), "$1/"+c.db+"$2")
	if c.pgPort != 0 {
		url = regexp.MustCompile(`@([^/:]+)(:\d+)?/`).ReplaceAllString(url, fmt.Sprintf("@$1:%d/", c.pgPort))
	}
	return url, nil
}

func createDB(c config) error {
	out, err := exec.Command("docker", "exec", c.container, "psql", "-U", c.pgUser, "-d", "postgres", "-tAc",
		"SELECT 1 FROM pg_database WHERE datname='"+c.db+"'").CombinedOutput()
	if err != nil {
		return fmt.Errorf("is the %s container running? %v: %s", c.container, err, out)
	}
	if strings.TrimSpace(string(out)) == "1" {
		return nil
	}
	if out, err := exec.Command("docker", "exec", c.container, "createdb", "-U", c.pgUser, c.db).CombinedOutput(); err != nil {
		return fmt.Errorf("createdb: %v: %s", err, out)
	}
	return nil
}

func recreateDB(c config) error {
	if out, err := exec.Command("docker", "exec", c.container, "dropdb", "-U", c.pgUser, "--if-exists", "--force", c.db).CombinedOutput(); err != nil {
		return fmt.Errorf("dropdb: %v: %s", err, out)
	}
	return createDB(c)
}

var testFuncRe = regexp.MustCompile(`^func (Test\w+)\(`)

// inventory reads the bound rules from cinch and resolves each marker to the
// test function under it. Markers can be stacked, several rules on one test,
// so comment lines between a marker and its func are skipped.
func inventory(c config, repo string) ([]rule, error) {
	cmd := exec.Command(c.cinch, "rules", "--json")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cinch rules --json: %w", err)
	}
	var doc struct {
		Rules []struct {
			ID, File, Text string
			Markers        []struct {
				File string
				Line int
			}
		}
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, err
	}
	module, err := goModule(filepath.Join(repo, "backend"))
	if err != nil {
		return nil, err
	}
	var rules []rule
	for _, d := range doc.Rules {
		if len(d.Markers) == 0 {
			continue
		}
		r := rule{ID: d.ID, Doc: d.File, Text: ruleText(d.Text)}
		seen := map[string]bool{}
		for _, m := range d.Markers {
			r.Markers = append(r.Markers, fmt.Sprintf("%s:%d", m.File, m.Line))
			t, err := testAt(repo, module, m.File, m.Line)
			if err != nil {
				return nil, fmt.Errorf("%s marker %s:%d: %w", d.ID, m.File, m.Line, err)
			}
			if !seen[t.key()] {
				seen[t.key()] = true
				r.Tests = append(r.Tests, t)
			}
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ruleText drops the doc text that `rules --json` carries past the end of a
// rule list's last item (the next heading and everything after it).
func ruleText(text string) string {
	if i := strings.Index(text, " ## "); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

func goModule(dir string) (string, error) {
	f, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(f)
	if m == nil {
		return "", fmt.Errorf("no module line in %s/go.mod", dir)
	}
	return string(m[1]), nil
}

func testAt(repo, module, file string, line int) (boundTest, error) {
	data, err := os.ReadFile(filepath.Join(repo, file))
	if err != nil {
		return boundTest{}, err
	}
	lines := strings.Split(string(data), "\n")
	start := line - 1
	// Include the whole comment block the marker sits in, so the model sees
	// the test's stated intent as well as its body.
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "//") {
		start--
	}
	i := line - 1
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "//") {
		i++
	}
	if i >= len(lines) {
		return boundTest{}, fmt.Errorf("no func after the marker")
	}
	m := testFuncRe.FindStringSubmatch(lines[i])
	if m == nil {
		return boundTest{}, fmt.Errorf("marker is not on a Test func: %q", lines[i])
	}
	end := i
	for end < len(lines) && lines[end] != "}" {
		end++
	}
	rel, err := filepath.Rel("backend", filepath.Dir(file))
	if err != nil {
		return boundTest{}, err
	}
	return boundTest{
		File: file, Name: m[1], Line: i + 1,
		Package: module + "/" + filepath.ToSlash(rel),
		Source:  strings.Join(lines[start:min(end+1, len(lines))], "\n"),
	}, nil
}

// baseline runs the unmutated suite c.baselineRuns times. A test failing in
// any run is flaky and excluded from grading; a package that fails outside
// any test means the fixture isn't runnable, so the measurement stops.
func baseline(c config, repo, testURL string) (map[string]bool, error) {
	flaky := map[string]bool{}
	for i := 1; i <= c.baselineRuns; i++ {
		logf("baseline run %d/%d", i, c.baselineRuns)
		out, _, err := goTest(c, repo, testURL)
		if err != nil {
			return nil, err
		}
		failed, pkgFails := failures(out)
		if len(pkgFails) > 0 {
			return nil, fmt.Errorf("packages fail outside any test at baseline: %s", strings.Join(pkgFails, ", "))
		}
		for _, t := range failed {
			flaky[t] = true
		}
		if err := os.MkdirAll(c.work, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(c.work, fmt.Sprintf("baseline-%s-%d.json", c.sha, i)), out, 0o644); err != nil {
			return nil, err
		}
	}
	if len(flaky) > 0 {
		names := keys(flaky)
		logf("excluded as flaky: %s", strings.Join(names, " "))
		if err := os.WriteFile(filepath.Join(c.work, "flaky-"+c.sha+".txt"), []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return flaky, nil
}

// goTest runs the suite against a freshly created database. The fixture's
// migrations run through goose, which never re-applies a version it has
// recorded, so a mutation to an existing migration only reaches the schema
// if the database starts empty.
func goTest(c config, repo, testURL string) ([]byte, int64, error) {
	return goTestArgs(c, repo, testURL, "./...")
}

func goTestArgs(c config, repo, testURL string, args ...string) ([]byte, int64, error) {
	if err := recreateDB(c); err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"test", "-json", "-count=1", "-p", "1"}, args...)...)
	cmd.Dir = filepath.Join(repo, "backend")
	cmd.Env = append(os.Environ(), "TEST_DATABASE_URL="+testURL)
	start := time.Now()
	out, err := cmd.Output()
	ms := time.Since(start).Milliseconds()
	if ctx.Err() != nil {
		return out, ms, fmt.Errorf("go test timed out after %s", c.testTimeout)
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return out, ms, err
	}
	return out, ms, nil // a non-zero exit is just failing tests
}

// failures reads go test -json output. It returns the failed top-level tests
// as package.TestName and the packages that failed with no test failing. A
// test that started but never finished (a panic or a timeout kills the
// binary) counts as failed: it is the one that was running.
func failures(out []byte) (tests []string, packages []string) {
	type event struct{ Action, Package, Test string }
	started := map[string]bool{}
	done := map[string]bool{}
	failed := map[string]bool{}
	pkgFailed := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Test == "" {
			if e.Action == "fail" {
				pkgFailed[e.Package] = true
			}
			continue
		}
		name := e.Package + "." + strings.SplitN(e.Test, "/", 2)[0]
		switch e.Action {
		case "run":
			started[name] = true
		case "pass", "skip":
			done[name] = true
		case "fail":
			done[name], failed[name] = true, true
		}
	}
	for name := range started {
		if !done[name] {
			failed[name] = true
		}
	}
	for name := range failed {
		pkgFailed[name[:strings.LastIndex(name, ".")]] = false
	}
	for pkg, unexplained := range pkgFailed {
		if unexplained {
			packages = append(packages, pkg)
		}
	}
	sort.Strings(packages)
	return keys(failed), packages
}

func measure(c config, repo, testURL, dir string, r rule, flaky map[string]bool) (result, error) {
	res := result{ID: r.ID, Doc: r.Doc, Markers: r.Markers, Usage: usage{Tools: map[string]int{}}}
	for _, t := range r.Tests {
		res.BoundTests = append(res.BoundTests, t.key())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	if err := resetTree(repo); err != nil {
		return res, err
	}
	defer resetTree(repo)

	if c.applyRule != "" {
		res.Applied, res.Mutation = true, "hand-written control diff "+filepath.Base(c.applyDiff)
		if _, err := git(repo, "apply", c.applyDiff); err != nil {
			return res, err
		}
	} else {
		prompt := fill(promptTemplate, r)
		if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
			return res, err
		}
		if err := propose(c, repo, filepath.Join(dir, "pi.jsonl"), prompt, &res); err != nil {
			return res, err
		}
	}

	diff, files, lines, reason, err := mutationDiff(repo)
	if err != nil {
		return res, err
	}
	res.Files, res.LinesChanged = files, lines
	if reason != "" {
		res.Outcome, res.Reason = "invalid", reason
		return res, os.WriteFile(filepath.Join(dir, "mutation.diff"), diff, 0o644)
	}

	buildErr := build(repo)
	if buildErr != "" && c.applyRule == "" {
		res.Retried = true
		prompt := strings.ReplaceAll(fill(retryTemplate, r), "{{BUILD_ERROR}}", truncate(buildErr, 4000))
		if err := propose(c, repo, filepath.Join(dir, "pi-retry.jsonl"), prompt, &res); err != nil {
			return res, err
		}
		diff, files, lines, reason, err = mutationDiff(repo)
		if err != nil {
			return res, err
		}
		res.Files, res.LinesChanged = files, lines
		if reason != "" {
			res.Outcome, res.Reason = "invalid", "after retry: "+reason
			return res, os.WriteFile(filepath.Join(dir, "mutation.diff"), diff, 0o644)
		}
		buildErr = build(repo)
	}
	if err := os.WriteFile(filepath.Join(dir, "mutation.diff"), diff, 0o644); err != nil {
		return res, err
	}
	if buildErr != "" {
		res.Outcome, res.Reason = "nocompile", truncate(buildErr, 500)
		return res, os.WriteFile(filepath.Join(dir, "build.err"), []byte(buildErr), 0o644)
	}

	out, ms, err := goTest(c, repo, testURL)
	res.TestMS = ms
	if werr := os.WriteFile(filepath.Join(dir, "test.json"), out, 0o644); werr != nil {
		return res, werr
	}
	if err != nil {
		return res, err
	}
	if out, err = withUnrunBound(c, repo, testURL, dir, out, res.BoundTests); err != nil {
		return res, err
	}
	res.FailingTests = graded(out, flaky)
	res.Outcome = grade(res.FailingTests, res.BoundTests)
	return res, nil
}

// regradeRule re-applies a rule's stored mutation and grades it again. It is
// for runs graded by a harness defect, so the mutation the model proposed is
// kept exactly; only the test run and its grade are redone.
func regradeRule(c config, repo, testURL, dir string, flaky map[string]bool) (result, error) {
	var res result
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return res, err
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return res, err
	}
	if res.Outcome == "invalid" || res.Outcome == "nocompile" {
		return res, fmt.Errorf("outcome %s has no test run to redo", res.Outcome)
	}
	if res.Regraded == "" {
		res.Regraded = res.Outcome
	}
	if err := resetTree(repo); err != nil {
		return res, err
	}
	defer resetTree(repo)
	if _, err := git(repo, "apply", filepath.Join(dir, "mutation.diff")); err != nil {
		return res, err
	}
	if msg := build(repo); msg != "" {
		return res, fmt.Errorf("stored mutation no longer builds: %s", msg)
	}
	out, ms, err := goTest(c, repo, testURL)
	res.TestMS = ms
	if werr := os.WriteFile(filepath.Join(dir, "test.json"), out, 0o644); werr != nil {
		return res, werr
	}
	if err != nil {
		return res, err
	}
	if out, err = withUnrunBound(c, repo, testURL, dir, out, res.BoundTests); err != nil {
		return res, err
	}
	res.FailingTests = graded(out, flaky)
	res.Outcome = grade(res.FailingTests, res.BoundTests)
	return res, nil
}

// withUnrunBound reruns, alone, any bound test the suite never finished. A
// panic kills its package's test binary, so the tests after it never run,
// and a bound test that never ran can't count as having missed the mutation.
// The isolated run's events are appended to the suite's for grading.
func withUnrunBound(c config, repo, testURL, dir string, out []byte, bound []string) ([]byte, error) {
	done := finished(out)
	byPkg := map[string][]string{}
	for _, b := range bound {
		if !done[b] {
			i := strings.LastIndex(b, ".")
			byPkg[b[:i]] = append(byPkg[b[:i]], b[i+1:])
		}
	}
	if len(byPkg) == 0 {
		return out, nil
	}
	module, err := goModule(filepath.Join(repo, "backend"))
	if err != nil {
		return nil, err
	}
	var extra []byte
	for _, pkg := range keys(boolSet(byPkg)) {
		names := byPkg[pkg]
		logf("  rerunning unfinished bound tests alone: %s", strings.Join(names, " "))
		run, _, err := goTestArgs(c, repo, testURL, "-run", "^("+strings.Join(names, "|")+")$", "./"+strings.TrimPrefix(pkg, module+"/"))
		if err != nil {
			return nil, err
		}
		extra = append(extra, run...)
	}
	if err := os.WriteFile(filepath.Join(dir, "test-bound.json"), extra, 0o644); err != nil {
		return nil, err
	}
	return append(append([]byte{}, out...), extra...), nil
}

func boolSet(m map[string][]string) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// finished returns the top-level tests that reached pass, fail or skip.
func finished(out []byte) map[string]bool {
	done := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e struct{ Action, Package, Test string }
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Test == "" || strings.Contains(e.Test, "/") {
			continue
		}
		if e.Action == "pass" || e.Action == "fail" || e.Action == "skip" {
			done[e.Package+"."+e.Test] = true
		}
	}
	return done
}

// graded lists the failures go test -json reports, flaky tests excluded.
func graded(out []byte, flaky map[string]bool) []string {
	var failing []string
	failed, pkgFails := failures(out)
	for _, t := range failed {
		if !flaky[t] {
			failing = append(failing, t)
		}
	}
	for _, p := range pkgFails {
		failing = append(failing, p+" (package)")
	}
	return failing
}

// grade is the pre-registered classifier: marked if a bound test failed,
// other if only unbound tests failed, silent if nothing did.
func grade(failing, bound []string) string {
	if len(failing) == 0 {
		return "silent"
	}
	for _, f := range failing {
		for _, b := range bound {
			if f == b {
				return "marked"
			}
		}
	}
	return "other"
}

func fill(tmpl string, r rule) string {
	var tests strings.Builder
	for _, t := range r.Tests {
		fmt.Fprintf(&tests, "`%s:%d`, `%s`:\n\n```go\n%s\n```\n\n", t.File, t.Line, t.Name, t.Source)
	}
	return strings.NewReplacer(
		"{{RULE_ID}}", r.ID,
		"{{RULE_DOC}}", r.Doc,
		"{{RULE_TEXT}}", r.Text,
		"{{BOUND_TESTS}}", strings.TrimSpace(tests.String()),
	).Replace(tmpl)
}

// propose runs pi once with no shell: it can read the tree and edit files but
// can't run the tests, so it can't pick a mutation by whether it's caught.
func propose(c config, repo, logPath, prompt string, res *result) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.proposeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pi", "-p", "--mode", "json",
		"--provider", c.provider, "--model", c.model, "--thinking", c.thinking,
		"--no-session", "--offline", "-nc", "-na", "-ne", "-ns", "-np",
		"-t", "read,grep,find,ls,edit", prompt)
	cmd.Dir = repo
	// pi spawns its own children; kill the whole group on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = logFile, &stderr
	start := time.Now()
	err = cmd.Run()
	res.ProposeMS += time.Since(start).Milliseconds()
	if ctx.Err() != nil {
		res.Usage.TimedOut = true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		res.Usage.ExitCode = exit.ExitCode()
	} else if err != nil && ctx.Err() == nil {
		return fmt.Errorf("pi: %w", err)
	}
	if stderr.Len() > 0 {
		_ = os.WriteFile(strings.TrimSuffix(logPath, ".jsonl")+".err", stderr.Bytes(), 0o644)
	}
	text, err := readPiLog(logPath, &res.Usage)
	if err != nil {
		return err
	}
	if m := regexp.MustCompile(`(?m)^\s*\**MUTATION:?\**:?\s*(.+)$`).FindAllStringSubmatch(text, -1); m != nil {
		res.Mutation = strings.TrimSpace(m[len(m)-1][1])
	}
	return nil
}

// readPiLog sums usage over assistant message_end events (message_update and
// turn_end repeat them) and returns the last assistant text.
func readPiLog(path string, u *usage) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var e struct {
			Type     string
			ToolName string
			Message  struct {
				Role    string
				Content []struct{ Type, Text string }
				Usage   struct{ Input, Output, CacheRead int }
			}
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch e.Type {
		case "tool_execution_start":
			u.Tools[e.ToolName]++
		case "message_end":
			if e.Message.Role != "assistant" {
				continue
			}
			u.Input += e.Message.Usage.Input
			u.Output += e.Message.Usage.Output
			u.CacheRead += e.Message.Usage.CacheRead
			u.Turns++
			var texts []string
			for _, c := range e.Message.Content {
				if c.Type == "text" && c.Text != "" {
					texts = append(texts, c.Text)
				}
			}
			if len(texts) > 0 {
				last = strings.Join(texts, "\n")
			}
		}
	}
	return last, sc.Err()
}

var diffStatRe = regexp.MustCompile(`^(\d+|-)\t(\d+|-)\t(.+)$`)

// mutationDiff returns the tree's diff against HEAD, the files it touches, the
// lines it changes, and why it's invalid ("" when it's a usable mutation).
func mutationDiff(repo string) ([]byte, []string, int, string, error) {
	if _, err := git(repo, "add", "-A", "-N"); err != nil {
		return nil, nil, 0, "", err
	}
	diff, err := git(repo, "diff", "HEAD")
	if err != nil {
		return nil, nil, 0, "", err
	}
	stat, err := git(repo, "diff", "--numstat", "HEAD")
	if err != nil {
		return nil, nil, 0, "", err
	}
	var files []string
	lines := 0
	reason := ""
	for _, l := range strings.Split(strings.TrimSpace(stat), "\n") {
		m := diffStatRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		var a, d int
		fmt.Sscan(m[1], &a)
		fmt.Sscan(m[2], &d)
		lines += a + d
		f := m[3]
		files = append(files, f)
		switch {
		case !strings.HasPrefix(f, "backend/"):
			reason = "touches a file outside backend/: " + f
		case strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "backend/tests/"):
			reason = "touches a test file: " + f
		}
	}
	if len(files) == 0 {
		reason = "no change"
	}
	return []byte(diff), files, lines, reason, nil
}

// build compiles everything go test will, test packages included, and
// returns the compiler output on failure.
func build(repo string) string {
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = filepath.Join(repo, "backend")
		if out, err := cmd.CombinedOutput(); err != nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func writeOutputs(c config, runs string, all []rule) error {
	if err := os.MkdirAll(c.mutations, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"id", "doc", "markers", "bound_tests", "outcome", "failing_tests", "retried",
		"files", "lines_changed", "propose_s", "test_s", "mutation", "reason", "regraded_from"})
	for _, r := range all {
		var res result
		data, err := os.ReadFile(filepath.Join(runs, r.ID, "meta.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &res); err != nil {
			return err
		}
		diff, err := os.ReadFile(filepath.Join(runs, r.ID, "mutation.diff"))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(c.mutations, r.ID+".diff"), diff, 0o644); err != nil {
			return err
		}
		w.Write([]string{res.ID, res.Doc, strings.Join(res.Markers, " "), strings.Join(shortTests(res.BoundTests), " "),
			res.Outcome, strings.Join(shortTests(res.FailingTests), " "), fmt.Sprint(res.Retried),
			strings.Join(res.Files, " "), fmt.Sprint(res.LinesChanged),
			fmt.Sprint(res.ProposeMS / 1000), fmt.Sprint(res.TestMS / 1000), res.Mutation, res.Reason, res.Regraded})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return os.WriteFile(c.out, buf.Bytes(), 0o644)
}

// shortTests drops the module prefix from package.Test names for the CSV.
func shortTests(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		if j := strings.Index(n, "/tests/"); j >= 0 {
			n = n[j+len("/tests/"):]
		}
		out[i] = n
	}
	return out
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…"
}
