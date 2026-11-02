package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// rules builds a rule inventory from id -> text, with one marker under test/
// unless the id is listed in unmarked, or in libMarked (marker outside test/).
func rules(texts map[string]string, unmarked, libMarked []string) map[string]corpusRule {
	out := map[string]corpusRule{}
	for id, text := range texts {
		rule := corpusRule{ID: id, Text: text}
		file := "test/" + id + "_test.dart"
		switch {
		case contains(unmarked, id):
			out[id] = rule
			continue
		case contains(libMarked, id):
			file = "lib/" + id + ".dart"
		}
		rule.Markers = append(rule.Markers, struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}{file, 1})
		out[id] = rule
	}
	return out
}

func TestEvalKeyChange(t *testing.T) {
	pin := rules(map[string]string{"SMS-006": "Scans run once a minute.", "CAT-001": "Names are unique."}, nil, nil)
	k := key{Kind: "change", Rules: []keyRule{{ID: "SMS-006", Match: []string{"13"}, Forbid: []string{"once a minute"}}}}
	cases := []struct {
		name   string
		final  map[string]corpusRule
		reason string
	}{
		{"pass", rules(map[string]string{"SMS-006": "Scans run every 13 minutes.", "CAT-001": "Names are unique."}, nil, nil), ""},
		{"stale text", rules(map[string]string{"SMS-006": "Scans run once a minute, 13 times.", "CAT-001": "x"}, nil, nil), "SMS-006: text fails match/forbid"},
		{"missing token", rules(map[string]string{"SMS-006": "Scans run rarely.", "CAT-001": "x"}, nil, nil), "SMS-006: text fails match/forbid"},
		{"rule missing", rules(map[string]string{"CAT-001": "x"}, nil, nil), "SMS-006: missing"},
		{"marker lost", rules(map[string]string{"SMS-006": "every 13 minutes", "CAT-001": "x"}, []string{"SMS-006"}, nil), "SMS-006: no marker"},
		{"other rule lost", rules(map[string]string{"SMS-006": "every 13 minutes"}, nil, nil), "CAT-001: lost (absent or unmarked)"},
		{"other rule unmarked", rules(map[string]string{"SMS-006": "every 13 minutes", "CAT-001": "x"}, []string{"CAT-001"}, nil), "CAT-001: lost (absent or unmarked)"},
		{"case insensitive", rules(map[string]string{"SMS-006": "EVERY 13 MINUTES", "CAT-001": "x"}, nil, nil), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reasons, err := evalKey(k, pin, c.final)
			if err != nil {
				t.Fatal(err)
			}
			if got := contains(reasons, c.reason); c.reason == "" && len(reasons) != 0 || c.reason != "" && !got {
				t.Errorf("reasons = %v, want %q", reasons, c.reason)
			}
		})
	}
}

func TestEvalKeyRetire(t *testing.T) {
	pin := rules(map[string]string{"CAT-002": "Colors round-trip.", "XFER-005": "Derived values are never exported."}, nil, nil)
	k := key{
		Kind:   "change",
		Rules:  []keyRule{{ID: "XFER-005", Match: []string{"never exported"}}},
		Retire: []keyRetire{{ID: "CAT-002", IfPresent: &keyCondition{Match: []string{"name"}, Forbid: []string{"round-trip"}}}},
	}
	cases := []struct {
		name  string
		final map[string]corpusRule
		fail  bool
	}{
		{"retired", rules(map[string]string{"XFER-005": "Derived values are never exported."}, nil, nil), false},
		{"rewritten", rules(map[string]string{"CAT-002": "A color follows the name.", "XFER-005": "Derived values are never exported."}, nil, nil), false},
		{"still claims round trip", rules(map[string]string{"CAT-002": "Colors round-trip by name.", "XFER-005": "Derived values are never exported."}, nil, nil), true},
		{"rewritten but unmarked is fine", rules(map[string]string{"CAT-002": "A color follows the name.", "XFER-005": "Derived values are never exported."}, []string{"CAT-002"}, nil), false},
		{"other rule lost", rules(map[string]string{"CAT-002": "A color follows the name."}, nil, nil), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reasons, err := evalKey(k, pin, c.final)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(reasons) > 0; got != c.fail {
				t.Errorf("fails = %t (%v), want %t", got, reasons, c.fail)
			}
		})
	}
	t.Run("retire without ifPresent means absent", func(t *testing.T) {
		strict := key{Retire: []keyRetire{{ID: "CAT-002"}}}
		reasons, _ := evalKey(strict, pin, pin)
		if len(reasons) == 0 {
			t.Error("a present rule passed a retire with no ifPresent")
		}
	})
}

func TestEvalKeyNewRule(t *testing.T) {
	pin := rules(map[string]string{"ENTRY-005": "Entries are kept."}, nil, nil)
	k := key{Kind: "add", NewRule: &keyCondition{Match: []string{"seal"}}}
	cases := []struct {
		name  string
		final map[string]corpusRule
		fail  bool
	}{
		{"new marked rule", rules(map[string]string{"ENTRY-005": "Entries are kept.", "ENTRY-006": "A sealed category takes no writes."}, nil, nil), false},
		{"marker only outside test/", rules(map[string]string{"ENTRY-005": "Entries are kept.", "ENTRY-006": "A sealed category takes no writes."}, nil, []string{"ENTRY-006"}), true},
		{"unmarked", rules(map[string]string{"ENTRY-005": "Entries are kept.", "ENTRY-006": "A sealed category takes no writes."}, []string{"ENTRY-006"}, nil), true},
		{"wrong text", rules(map[string]string{"ENTRY-005": "Entries are kept.", "ENTRY-006": "Entries are dated."}, nil, nil), true},
		{"old rule rewritten is not new", rules(map[string]string{"ENTRY-005": "A sealed entry is kept."}, nil, nil), true},
		{"none", rules(map[string]string{"ENTRY-005": "Entries are kept."}, nil, nil), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reasons, err := evalKey(k, pin, c.final)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(reasons) > 0; got != c.fail {
				t.Errorf("fails = %t (%v), want %t", got, reasons, c.fail)
			}
		})
	}
}

func TestEvalKeyLeave(t *testing.T) {
	pin := rules(map[string]string{"MONEY-002": "Formatting never changes stored values.", "CAT-001": "Names are unique."}, nil, nil)
	k := key{Kind: "leave", Rules: []keyRule{{ID: "MONEY-002", Match: []string{"never changes stored values"}}}}
	same := rules(map[string]string{"MONEY-002": "Formatting never changes stored values.", "CAT-001": "Names are unique."}, nil, nil)
	if reasons, _ := evalKey(k, pin, same); len(reasons) != 0 {
		t.Errorf("untouched corpus fails: %v", reasons)
	}
	edited := rules(map[string]string{"MONEY-002": "Formatting, including roman numerals, never alters amounts.", "CAT-001": "Names are unique."}, nil, nil)
	if reasons, _ := evalKey(k, pin, edited); len(reasons) == 0 {
		t.Error("a rewrite that lost the token passed")
	}
	if got := changedRules(pin, edited); !reflect.DeepEqual(got, []string{"MONEY-002"}) {
		t.Errorf("changedRules = %v", got)
	}
}

func TestEvalKeyBadPattern(t *testing.T) {
	k := key{Rules: []keyRule{{ID: "A-1", Match: []string{"("}}}}
	r := rules(map[string]string{"A-1": "x"}, nil, nil)
	if _, err := evalKey(k, r, r); err == nil {
		t.Error("an invalid pattern was accepted")
	}
}

func TestScheduleDeterministic(t *testing.T) {
	a := schedule(20261028, "pi", canonicalTasks, 5)
	b := schedule(20261028, "pi", canonicalTasks, 5)
	if !reflect.DeepEqual(a, b) {
		t.Error("same seed and engine gave different orders")
	}
	if len(a) != 6*3*10 {
		t.Fatalf("schedule has %d runs, want 180", len(a))
	}
	if reflect.DeepEqual(a, schedule(20261028, "claude", canonicalTasks, 5)) {
		t.Error("two engines share one order")
	}
	if reflect.DeepEqual(a, schedule(7, "pi", canonicalTasks, 5)) {
		t.Error("two seeds share one order")
	}
	// The order is pre-registered: a change to the shuffle changes it, so pin it.
	if got := []string{a[0].id(), a[1].id(), a[2].id()}; !reflect.DeepEqual(got, wantFirstJobs) {
		t.Errorf("first runs = %v, want %v", got, wantFirstJobs)
	}
}

var wantFirstJobs = []string{"C3-arm3-r8", "S3-arm3-r7", "S1-arm2-r2"}

func TestStageSplit(t *testing.T) {
	all := schedule(1, "pi", canonicalTasks, 5)
	one := selectJobs(all, 1, 5, nil, nil)
	two := selectJobs(all, 2, 5, nil, nil)
	if len(one) != 90 || len(two) != 90 {
		t.Fatalf("stages have %d and %d runs, want 90 each", len(one), len(two))
	}
	for _, j := range one {
		if j.Index > 4 {
			t.Errorf("stage 1 holds %s", j.id())
		}
	}
	for _, j := range two {
		if j.Index < 5 {
			t.Errorf("stage 2 holds %s", j.id())
		}
	}
	// Both stages are subsequences of the one shuffle.
	last := -1
	for _, j := range append(append([]job{}, one...), two...) {
		if all[j.Position].id() != j.id() {
			t.Fatalf("%s moved from its position", j.id())
		}
	}
	for _, j := range one {
		if j.Position <= last {
			t.Fatal("stage 1 is out of shuffle order")
		}
		last = j.Position
	}
	counts := map[string]int{}
	for _, j := range one {
		counts[j.Task+string(rune('0'+j.Arm))]++
	}
	for cell, n := range counts {
		if n != 5 {
			t.Errorf("cell %s has %d runs in stage 1, want 5", cell, n)
		}
	}
	subset := selectJobs(all, 1, 5, []string{"C1"}, []int{2})
	if len(subset) != 5 {
		t.Errorf("C1 arm 2 stage 1 has %d runs, want 5", len(subset))
	}
}

func TestDeriveAgents(t *testing.T) {
	pin, err := os.ReadFile(filepath.Join("testdata", "pin-AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, arm := range []int{1, 2} {
		t.Run("arm"+string(rune('0'+arm)), func(t *testing.T) {
			got, err := os.ReadFile(filepath.Join("arms", "arm"+string(rune('0'+arm)), "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			if want := deriveAgents(string(pin), arm); string(got) != want {
				t.Errorf("arms/arm%d/AGENTS.md is not what the pin derives to:\n%s\n---\n%s", arm, got, want)
			}
		})
	}
	arm1, arm2 := deriveAgents(string(pin), 1), deriveAgents(string(pin), 2)
	if strings.Contains(arm1, "docs") && strings.Contains(arm1, "Business rules") {
		t.Error("arm 1 kept the docs section")
	}
	for _, command := range []string{"cinch index", "cinch workflows", "cinch check", "cinch workflow"} {
		if strings.Contains(arm1, command) || strings.Contains(arm2, command) {
			t.Errorf("an arm kept %q", command)
		}
	}
	for _, kept := range []string{"cinch:rule <ID>", "Changing a behavior a rule describes", "A new invariant", "Business rules are numbered"} {
		if !strings.Contains(arm2, kept) {
			t.Errorf("arm 2 lost %q", kept)
		}
	}
	if !strings.Contains(arm1, "Budgetron is a Flutter app") || !strings.Contains(arm2, "Budgetron is a Flutter app") {
		t.Error("an arm lost the Project section")
	}
	if deriveAgents(string(pin), 3) != string(pin) {
		t.Error("arm 3 changed AGENTS.md")
	}
}

func TestStripMakeCheck(t *testing.T) {
	target, err := os.ReadFile(filepath.Join("testdata", "pin-check-target.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := stripMakeCheck(string(target))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "cinch check") || !strings.Contains(got, "$(MAKE) strings") {
		t.Errorf("check target after stripping:\n%s", got)
	}
	if _, err := stripMakeCheck("check:\n\ttrue\n"); err == nil {
		t.Error("a Makefile without the line was accepted")
	}
}

func sandboxHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	touch := func(name string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	return &harness{
		config:   config{provider: "llama-cpp", model: "m", thinking: "high"},
		cinchBin: touch("cinch"),
		sandbox: sandboxPaths{home: "/home/tester", flutter: root, pubCache: root, claudeBin: touch("claude"),
			credentials: touch("creds"), piNode: filepath.Join(root, "node", "bin", "node"), piModels: touch("models.json"),
			piCLI: filepath.Join(root, "npx", "node_modules", "@s", "pi", "dist", "bundle", "cli.js")},
	}
}

func argsContain(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		if reflect.DeepEqual(args[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

func TestSandboxArgs(t *testing.T) {
	h := sandboxHarness(t)
	rundir := t.TempDir()
	arms := map[int][]string{}
	for _, arm := range canonicalArms {
		args, err := h.sandboxArgs(sandboxSpec{arm: arm, repo: "/ws/abc/budgetron", rundir: rundir, engine: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		arms[arm] = args
		joined := strings.Join(args, " ")
		for _, leak := range []string{"/code", "cinch-upkeep", "/.cache", "/.claude/CLAUDE.md", "/.local"} {
			if strings.Contains(joined, leak) {
				t.Errorf("arm %d binds %q", arm, leak)
			}
		}
		if !argsContain(args, "--bind", "/ws/abc/budgetron", "/work") || !argsContain(args, "--tmpfs", "/home") || !argsContain(args, "--clearenv") {
			t.Errorf("arm %d lacks the checkout at /work, the empty home or a clean env", arm)
		}
		if argsContain(args, "--unshare-net") {
			t.Errorf("arm %d unshares the network", arm)
		}
		if !argsContain(args, "--ro-bind", h.sandbox.credentials, "/home/tester/.claude/.credentials.json") {
			t.Errorf("arm %d lacks the read-only credentials", arm)
		}
	}
	for _, arm := range []int{1, 2} {
		if strings.Contains(strings.Join(arms[arm], " "), "cinch") {
			t.Errorf("arm %d sandbox mentions cinch", arm)
		}
	}
	if !argsContain(arms[3], "--ro-bind", h.cinchBin, sandboxCinch+"/cinch") || !argsContain(arms[3], "--setenv", "PATH", sandboxCinch+":"+h.sandbox.flutter+"/bin:"+h.sandbox.flutter+"/bin/cache/dart-sdk/bin:/usr/local/bin:/usr/bin") {
		t.Errorf("arm 3 does not get the grader's cinch first on PATH: %v", arms[3])
	}
	// Apart from cinch, the three sandboxes are the same.
	strip := func(args []string) string {
		joined := " " + strings.Join(args, " ") + " "
		joined = strings.Replace(joined, " --ro-bind "+h.cinchBin+" "+sandboxCinch+"/cinch ", " ", 1)
		return strings.Replace(joined, sandboxCinch+":", "", 1)
	}
	if strip(arms[1]) != strip(arms[2]) || strip(arms[2]) != strip(arms[3]) {
		t.Error("the arms' sandboxes differ in more than cinch")
	}

	pi, err := h.sandboxArgs(sandboxSpec{arm: 2, repo: "/ws/x", rundir: rundir, engine: "pi"})
	if err != nil {
		t.Fatal(err)
	}
	if !argsContain(pi, "--ro-bind", h.sandbox.piModels, "/home/tester/.pi/agent/models.json") || strings.Contains(strings.Join(pi, " "), ".claude") {
		t.Errorf("pi sandbox: %v", pi)
	}
	settings, _ := os.ReadFile(filepath.Join(rundir, "pi-settings.json"))
	if !strings.Contains(string(settings), `"defaultModel":"m"`) || strings.Contains(string(settings), "packages") {
		t.Errorf("pi settings = %s", settings)
	}
}

// TestSandboxProbe runs a script, standing in for a model, inside each arm's
// real sandbox and checks what it can and cannot see.
func TestSandboxProbe(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	flutter := filepath.Join(home, "dev", "flutter")
	if _, err := os.Stat(bwrapPath); err != nil {
		t.Skip("no bubblewrap")
	}
	if _, err := os.Stat(flutter); err != nil {
		t.Skip("no Flutter SDK")
	}
	design, err := filepath.Abs("DESIGN.md")
	if err != nil {
		t.Fatal(err)
	}
	h := sandboxHarness(t)
	h.sandbox = sandboxPaths{home: home, flutter: flutter, pubCache: filepath.Join(home, ".pub-cache")}
	probes := []struct{ name, command string }{
		{"cinch", "command -v cinch"},
		{"code", "ls ~/code"},
		{"abs", "cat " + design},
		{"cache", "ls ~/.cache/cinch-upkeep"},
		{"claude-md", "cat ~/.claude/CLAUDE.md"},
		{"dot-local", "ls ~/.local"},
		{"toolchain", "command -v flutter git make"},
		{"workspace", "echo hi > /work/probe && cat /work/probe"},
	}
	var script strings.Builder
	for _, p := range probes {
		fmt.Fprintf(&script, "if (%s) >/dev/null 2>&1; then echo '%s ok'; else echo '%s no'; fi\n", p.command, p.name, p.name)
	}
	for _, arm := range canonicalArms {
		repo := t.TempDir()
		cmd, err := h.sandboxCmd(sandboxSpec{arm: arm, repo: repo, rundir: t.TempDir(), engine: "fake"}, "/bin/bash", "-c", script.String())
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("arm %d: %v: %s", arm, err, out)
		}
		got := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			name, verdict, _ := strings.Cut(line, " ")
			got[name] = verdict == "ok"
		}
		for _, p := range probes {
			want := p.name == "toolchain" || p.name == "workspace" || (p.name == "cinch" && arm == 3)
			if got[p.name] != want {
				t.Errorf("arm %d probe %q = %t, want %t\n%s", arm, p.name, got[p.name], want, out)
			}
		}
	}
}

func TestCommitMsgHook(t *testing.T) {
	manifest := "commit:\n  # Canonical types\n  pattern: '^(merge: ((feature|fix|refactor|chore|docs)/.+|dev$)|(feat|fix|ui|docs|style|refactor|test|chore|perf)(!)?( \\[[a-z+-]+\\])?: .+)'\n"
	pattern, err := commitPatternOf([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := exec.Command("git", "init", "-q", dir)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	hook := filepath.Join(dir, "commit-msg")
	if err := os.WriteFile(hook, []byte(hookScript("commit-msg", pattern)), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		subject string
		ok      bool
	}{
		{"fix [sms]: handle no entries", true},
		{"feat: add x", true},
		{"merge: feature/x", true},
		{"merge: dev", true},
		{"fix stuff", false},
		{"Merge branch 'x'", false},
		{"feat:no space", false},
		{"it's: broken", false},
	}
	for _, c := range cases {
		t.Run(c.subject, func(t *testing.T) {
			message := filepath.Join(dir, "MSG")
			if err := os.WriteFile(message, []byte(c.subject+"\n\nbody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(hook, message)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if (err == nil) != c.ok {
				t.Errorf("ok = %t, want %t: %s", err == nil, c.ok, out)
			}
			if !c.ok && !strings.Contains(string(out), "commit message does not match commit.pattern ("+pattern+"): "+c.subject) {
				t.Errorf("rejection text: %s", out)
			}
		})
	}
	log, _ := os.ReadFile(filepath.Join(dir, ".git", hookLogName))
	if strings.Count(string(log), "commit-msg\t0") != 4 || strings.Count(string(log), "commit-msg\t1") != 4 {
		t.Errorf("hook log:\n%s", log)
	}
	quoted, err := commitPatternOf([]byte("  pattern: 'a''b'\n"))
	if err != nil || quoted != "a'b" {
		t.Errorf("quoted pattern = %q, %v", quoted, err)
	}
}

func TestScanBypass(t *testing.T) {
	bash := func(command string) []toolCall { return []toolCall{{Name: "bash", Command: command}} }
	cases := []struct {
		name  string
		calls []toolCall
		want  []string
	}{
		{"clean commit", bash(`git add -A && git commit -m "feat: x"`), nil},
		{"no-verify", bash(`git commit --no-verify -m "feat: x"`), []string{"no-verify"}},
		{"commit -n", bash(`git commit -n -m "feat: x"`), []string{"commit-n"}},
		{"commit -an", bash(`git commit -an -m x`), []string{"commit-n"}},
		{"commit -nm", bash(`git commit -nm x`), []string{"commit-n"}},
		{"message starting -n is not a flag", bash(`git commit -m "-n flag handling"`), nil},
		{"message mentioning no-verify", bash(`git commit -m "document --no-verify"`), nil},
		{"git -C commit -n", bash(`git -C sub commit -n`), []string{"commit-n"}},
		{"hooksPath set", bash(`git config core.hooksPath /dev/null`), []string{"hooksPath"}},
		{"hooksPath read", bash(`git config core.hooksPath`), nil},
		{"hooksPath -c", bash(`git -c core.hooksPath=/dev/null commit -m x`), []string{"hooksPath"}},
		{"hooksPath unset", bash(`git config --unset core.hooksPath`), []string{"hooksPath"}},
		{"hook rm", bash(`rm .githooks/pre-commit`), []string{"hook-edit"}},
		{"hook chmod", bash(`chmod -x .git/hooks/pre-commit`), []string{"hook-edit"}},
		{"hook cat", bash(`cat .githooks/pre-commit 2>&1`), nil},
		{"hook ls", bash(`ls -la .git/hooks`), nil},
		{"edit tool", []toolCall{{Name: "edit", Path: "/w/budgetron/.githooks/pre-commit"}}, []string{"hook-edit"}},
		{"read tool", []toolCall{{Name: "read", Path: "/w/budgetron/.githooks/pre-commit"}}, nil},
		{"chained", bash(`make check; git commit --no-verify -m x`), []string{"no-verify"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scanBypass(c.calls); !reflect.DeepEqual(got, c.want) {
				t.Errorf("scanBypass = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRunsCinch(t *testing.T) {
	cases := map[string]bool{
		"cinch check":                    true,
		"make check && cinch check":      true,
		"~/go/bin/cinch rules":           true,
		"FOO=1 cinch index":              true,
		"sudo cinch check":               true,
		"which cinch":                    false,
		"command -v cinch":               false,
		"cat cinch.yml":                  false,
		"grep -r cinch:rule test":        false,
		"cd ~/code/cinch && ls":          false,
		`echo "run cinch check"`:         false,
		`git commit -m "cinch check ok"`: false,
	}
	for command, want := range cases {
		if got := runsCinch(command); got != want {
			t.Errorf("runsCinch(%q) = %t, want %t", command, got, want)
		}
	}
}

func TestParseTranscriptPi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	lines := `{"type":"session"}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input":10,"output":5,"cacheRead":2,"cost":{"total":0}},"stopReason":"toolUse"}}
{"type":"tool_execution_start","toolCallId":"a","toolName":"bash","args":{"command":"make check"}}
{"type":"tool_execution_end","toolCallId":"a","toolName":"bash","result":{"content":[{"type":"text","text":"ok"}]}}
{"type":"message_update","message":{"role":"assistant"}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input":20,"output":7,"cacheRead":3},"stopReason":"stop"}}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := parseTranscript("pi", path)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Turns != 2 || tr.Input != 30 || tr.Output != 12 || tr.CacheRead != 5 || tr.FinalText != "done" || tr.EngineError != "" {
		t.Errorf("trace = %+v", tr)
	}
	if len(tr.Calls) != 1 || tr.Calls[0].Command != "make check" || tr.Calls[0].Result != "ok" {
		t.Errorf("calls = %+v", tr.Calls)
	}
	errored := `{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"connection refused"}}` + "\n"
	if err := os.WriteFile(path, []byte(errored), 0o644); err != nil {
		t.Fatal(err)
	}
	if tr, _ = parseTranscript("pi", path); !strings.Contains(tr.EngineError, "connection refused") {
		t.Errorf("EngineError = %q", tr.EngineError)
	}
}

func TestParseTranscriptClaude(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	lines := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"git commit -n -m x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"committed"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"Edit","input":{"file_path":"lib/a.dart"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"edited"}]}]}}
{"type":"result","subtype":"success","is_error":false,"num_turns":3,"total_cost_usd":0.5,"result":"done","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":900,"cache_creation_input_tokens":40}}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := parseTranscript("claude", path)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Turns != 3 || tr.Input != 100 || tr.Output != 50 || tr.CacheRead != 900 || tr.CacheWrite != 40 || tr.CostUSD != 0.5 || tr.EngineError != "" {
		t.Errorf("trace = %+v", tr)
	}
	if len(tr.Calls) != 2 || tr.Calls[0].Result != "committed" || tr.Calls[1].Result != "edited" || tr.Calls[1].Path != "lib/a.dart" {
		t.Errorf("calls = %+v", tr.Calls)
	}
	if got := scanBypass(tr.Calls); !reflect.DeepEqual(got, []string{"commit-n"}) {
		t.Errorf("scanBypass = %v", got)
	}
	for subtype, wantError := range map[string]bool{"error_max_turns": false, "error_during_execution": true} {
		line := `{"type":"result","subtype":"` + subtype + `","is_error":true,"num_turns":80,"result":"x"}` + "\n"
		if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		tr, _ := parseTranscript("claude", path)
		if got := tr.EngineError != ""; got != wantError {
			t.Errorf("%s: engine error = %t, want %t", subtype, got, wantError)
		}
	}
	if err := os.WriteFile(path, []byte(`{"type":"system"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tr, _ := parseTranscript("claude", path); tr.EngineError == "" {
		t.Error("a transcript with no result line is not an engine error")
	}
}

func TestCountFlutterTests(t *testing.T) {
	out := []byte(`{"type":"testDone","result":"success","hidden":true,"skipped":false}
{"type":"testDone","result":"success","hidden":false,"skipped":false}
{"type":"testDone","result":"failure","hidden":false,"skipped":false}
{"type":"testDone","result":"success","hidden":false,"skipped":true}
{"type":"testDone","result":"error","hidden":true,"skipped":false}
not json
`)
	if passed, failed := countFlutterTests(out); passed != 1 || failed != 2 {
		t.Errorf("passed, failed = %d, %d, want 1, 2", passed, failed)
	}
}

func TestValidity(t *testing.T) {
	h := &harness{config: config{engine: "pi"}}
	cases := []struct {
		name    string
		res     result
		tr      trace
		ended   agentRun
		invalid bool
		note    bool
	}{
		{"clean", result{Arm: 2}, trace{Calls: []toolCall{{Name: "bash", Command: "make check", Input: `{"command":"make check"}`}}}, agentRun{Ran: true}, false, false},
		{"reads bench", result{Arm: 3}, trace{Calls: []toolCall{{Name: "read", Input: `{"path":"/home/u/code/cinch/bench/upkeep/DESIGN.md"}`}}}, agentRun{Ran: true}, true, false},
		{"similar path", result{Arm: 3}, trace{Calls: []toolCall{{Name: "read", Input: `{"path":"/home/u/code/cinch/benchmarks.md"}`}}}, agentRun{Ran: true}, false, false},
		{"cinch ran in arm 2", result{Arm: 2}, trace{Calls: []toolCall{{Name: "bash", Command: "cinch check", Result: "ok", Input: `{"command":"cinch check"}`}}}, agentRun{Ran: true}, true, false},
		{"cinch not found in arm 2", result{Arm: 2}, trace{Calls: []toolCall{{Name: "bash", Command: "cinch check", Result: "bash: cinch: command not found", Input: `{"command":"cinch check"}`}}}, agentRun{Ran: true}, false, true},
		{"cinch in arm 3", result{Arm: 3}, trace{Calls: []toolCall{{Name: "bash", Command: "cinch check", Result: "ok", Input: `{"command":"cinch check"}`}}}, agentRun{Ran: true}, false, false},
		{"engine error before edit", result{Arm: 2}, trace{EngineError: "connection refused"}, agentRun{Ran: true, ExitCode: 1}, true, false},
		{"engine error after edit", result{Arm: 2}, trace{EngineError: "x", Calls: []toolCall{{Name: "edit", Input: `{}`}}}, agentRun{Ran: true, ExitCode: 1}, false, false},
		{"timeout is not an engine error", result{Arm: 2}, trace{}, agentRun{Ran: true, ExitCode: -1, TimedOut: true}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h.validity(&c.res, &c.tr, c.ended, "/ws/x/budgetron")
			if got := len(c.res.Invalid) > 0; got != c.invalid {
				t.Errorf("invalid = %t (%v), want %t", got, c.res.Invalid, c.invalid)
			}
			if got := len(c.res.Notes) > 0; got != c.note {
				t.Errorf("notes = %v, want note %t", c.res.Notes, c.note)
			}
		})
	}
}

// TestKeyAmendments pins the texts checked for the key amendments
// (tasks/S1/NOTES.md, tasks/S2/NOTES.md, tasks/C1/NOTES.md) to the committed
// keys, and the grader's retirement rule (amendment 2).
func TestKeyAmendments(t *testing.T) {
	const sms = "6. **SMS-006** A background scan (app start or resume) runs only when the read permission is already granted and a scan point exists; it never prompts and never asks for a start point. "
	const catPin = "2. **CAT-002** A category color string and its `Color` round-trip losslessly."
	xfer := map[string]string{
		"XFER-001": "1. **XFER-001** Export and import are lossless.",
		"XFER-004": "4. **XFER-004** A file from an older format version still imports.",
		"XFER-005": "5. **XFER-005** Derived values are never exported.",
	}
	withCat := func(text string) map[string]string {
		out := map[string]string{"CAT-002": text}
		for id, t := range xfer {
			out[id] = t
		}
		return out
	}
	cases := []struct {
		name, task, id, text string
		pass                 bool
	}{
		// Amendment 1.
		{"S1 pin", "S1", "SMS-006", sms + "Scans run at most once a minute and never concurrently.", false},
		{"S1 reference", "S1", "SMS-006", sms + "Scans run at most once every 13 minutes and never concurrently.", true},
		{"S1 arm2-r2", "S1", "SMS-006", "A background scan (app start or resume) runs only when the read permission is already granted and a scan point exists; it never prompts, never asks for a start point and runs at most once every 13 minutes. UI scans run at most once a minute. Scans never run concurrently, and a scan that fails still counts for these intervals.", true},
		{"S1 13 added, pin sentence kept", "S1", "SMS-006", "A background scan runs at most once every 13 minutes. Scans run at most once a minute and never concurrently.", true}, // failed under amendment 1; passes under amendment 2
		{"S1 background at one minute", "S1", "SMS-006", "A background scan runs at most once a minute; manual scans every 13 minutes.", true},                                     // failed under amendment 1; passes under amendment 2
		{"S1 other words", "S1", "SMS-006", "Background scans (app start or resume) need the read permission and a scan point, never prompt, and run at most once every 13 minutes; scans from the SMS page at most once a minute. Scans never overlap.", true},
		{"S2 pin", "S2", "CAT-001", "Category names are unique. Exact match, case-sensitive; adding or renaming to a taken name throws `DuplicateCategoryName` and leaves the stored rows unchanged.", false},
		{"S2 whitespace", "S2", "CAT-001", "Category names are unique ignoring whitespace: `Eating Out`, `EatingOut` and `Eating  Out` are one name. Case still counts.", true},
		{"S2 exact match kept", "S2", "CAT-001", "Category names are unique: an exact match once whitespace is removed, case-sensitive.", true},
		// Amendment 2, S1: the four runs the amendment-1 forbids still rejected.
		{"S1 arm2-r1", "S1", "SMS-006", sms + "A background scan runs at most once every 13 minutes after the last scan finished (a failed scan counts too), a foreground scan at most once a minute, and two scans never run at the same time.", true},
		{"S1 arm3-r0", "S1", "SMS-006", sms + "Scans never run concurrently; a background scan runs at most once every 13 minutes after the previous scan finished, whoever scanned last, and a page-opening scan (scanIfStale) at most once a minute.", true},
		{"S1 arm3-r3", "S1", "SMS-006", sms + "A background scan runs at most once every 13 minutes after the previous scan (of either kind) finished, other scans at most once a minute, and scans never run concurrently.", true},
		{"S1 arm3-r4", "S1", "SMS-006", sms + "Scans run at most once a minute and never concurrently; a background scan that would start less than 13 minutes after the last scan finished is skipped, whatever that scan's outcome.", true},
		// Amendment 2, C1: retirement by stating it.
		{"C1 pin", "C1", "CAT-002", catPin, false},
		{"C1 arm3-r3 retired in place", "C1", "CAT-002", "2. **CAT-002** *(retired)* A category color string and its `Color` round-trip losslessly. Categories no longer store a color; it is derived from the name (CAT-003). The string/`Color` helpers remain for account colors, which are still stored.", true},
		{"C1 retired, not a word match", "C1", "CAT-002", "2. **CAT-002** A category color string and its `Color` round-trip losslessly; unretired helpers remain.", false},
		{"C1 rewritten", "C1", "CAT-002", "2. **CAT-002** A category's color is derived from its name and never stored.", true},
	}
	for _, c := range cases {
		k, err := loadKey(filepath.Join("tasks", c.task, "key.json"))
		if err != nil {
			t.Fatal(err)
		}
		final := rules(map[string]string{c.id: c.text}, nil, nil)
		pin := final
		if c.task == "C1" {
			final, pin = rules(withCat(c.text), nil, nil), rules(withCat(catPin), nil, nil)
		}
		reasons, err := evalKey(k, pin, final)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(reasons) == 0; got != c.pass {
			t.Errorf("%s: pass = %t, want %t: %v", c.name, got, c.pass, reasons)
		}
	}
}

func TestRegrade(t *testing.T) {
	work, tasks := t.TempDir(), t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sha := "0123456789abcdef"
	ruleJSON := func(text string) string {
		return `{"rules":[{"id":"A-1","file":"d.md","text":"` + text + `","markers":[{"file":"test/a_test.dart","line":1}]}]}`
	}
	write(filepath.Join(work, "pin-rules-"+sha[:12]+".json"), ruleJSON("old"))
	write(filepath.Join(tasks, "T", "key.json"), `{"task":"T","kind":"change","rules":[{"id":"A-1","match":["new"]}]}`)
	run := filepath.Join(work, "runs", "pi", "T-arm2-r0")
	write(filepath.Join(run, "rules.json"), ruleJSON("new"))
	if err := writeJSON(filepath.Join(run, "result.json"), result{RunID: "T-arm2-r0", Task: "T", Arm: 2,
		Behavior: "pass", Corpus: "fail", CorpusReasons: []string{"A-1: text fails match/forbid"}, Upkeep: "fail"}); err != nil {
		t.Fatal(err)
	}
	arm1 := filepath.Join(work, "runs", "pi", "T-arm1-r0")
	if err := os.MkdirAll(arm1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(arm1, "result.json"), result{RunID: "T-arm1-r0", Task: "T", Arm: 1,
		Behavior: "pass", Corpus: "n/a", Upkeep: "n/a"}); err != nil {
		t.Fatal(err)
	}
	h := &harness{config: config{work: work, tasksDir: tasks, engine: "pi", out: filepath.Join(work, "out.csv")},
		pins: map[string]string{"FIXTURE_SHA": sha}}
	for pass := 0; pass < 2; pass++ { // a second regrade changes nothing and keeps the first grade
		if err := h.regradeAll(); err != nil {
			t.Fatal(err)
		}
		got, err := readResult(run)
		if err != nil {
			t.Fatal(err)
		}
		if got.Corpus != "pass" || got.Upkeep != "pass" || got.CorpusFirst != "fail" || got.UpkeepFirst != "fail" || len(got.CorpusReasons) != 0 {
			t.Errorf("pass %d: regraded %+v", pass, got)
		}
		if one, _ := readResult(arm1); one.Corpus != "n/a" || one.CorpusFirst != "" {
			t.Errorf("pass %d: arm 1 was regraded: %+v", pass, one)
		}
	}
	csv, err := os.ReadFile(h.out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csv), "corpus_first") || strings.Count(string(csv), "\n") != 4 {
		t.Errorf("table:\n%s", csv)
	}
}
