package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestStats(t *testing.T) {
	one := []stratum{{Task: "A", X3: 2, N3: 2, X2: 0, N2: 2}}
	z, p, zCC, pCC := cmh(one)
	if !near(z, math.Sqrt(3)) || !near(p, 0.0416) || !near(zCC, 0.866) || !near(pCC, 0.1932) {
		t.Errorf("cmh = %v %v %v %v", z, p, zCC, pCC)
	}
	if got := exact(one); !near(got, 1.0/6) {
		t.Errorf("exact(one) = %v, want 1/6", got)
	}
	// Two identical strata: P(S >= 4) = (1/6)^2.
	two := []stratum{one[0], {Task: "B", X3: 2, N3: 2, X2: 0, N2: 2}}
	if got := exact(two); !near(got, 1.0/36) {
		t.Errorf("exact(two) = %v, want 1/36", got)
	}
	// A stratum with no spread adds nothing.
	flat := append(two, stratum{Task: "C", X3: 5, N3: 5, X2: 5, N2: 5})
	if got := exact(flat); !near(got, 1.0/36) {
		t.Errorf("exact(flat) = %v, want 1/36", got)
	}
	ceil, rest := ceiling(flat)
	if len(ceil) != 1 || ceil[0].Task != "C" || len(rest) != 2 {
		t.Errorf("ceiling = %v / %v", ceil, rest)
	}
	if got := diffPoints(flat); !near(got, 100*(9.0/9-5.0/9)) {
		t.Errorf("diffPoints = %v", got)
	}
	if got := median([]float64{5, 1, 3, 2}); got != 2.5 {
		t.Errorf("median = %v", got)
	}
}

const header = "run_id,engine,task,arm,index,attempt,timed_out,committed,bypass,behavior,corpus,upkeep,suite,hook_blocks,turns,wall_s,output_tokens\n"

func TestReadRunsFinalAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.csv")
	body := "# engine pi\n" + header +
		"S1-arm3-r0,pi,S1,3,0,1,true,false,false,fail,fail,fail,fail,1,10,600,100\n" +
		"S1-arm3-r0,pi,S1,3,0,2,false,true,false,pass,pass,pass,pass,0,20,1200,300\n" +
		"S1-arm2-r0,pi,S1,2,0,1,false,true,false,pass,pass,pass,pass,0,30,60,50\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := readRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[1].ID != "S1-arm3-r0" || runs[1].Attempt != 2 || runs[1].Upkeep != "pass" || runs[1].WallMin != 20 {
		t.Fatalf("runs = %+v", runs)
	}
	var out bytes.Buffer
	report(&out, path, runs)
	for _, want := range []string{"2 runs", "pooled difference: +0.0 points", "ceiling tasks (100% upkeep in arms 2 and 3): S1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
}

func TestTranscriptUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := []string{
		`{"type":"message_update","note":"ignored"}`,
		`{"type":"tool_execution_start","toolName":"bash","args":{"command":"cd /work && cinch check && ls cinch.yml"}}`,
		`{"type":"tool_execution_start","toolName":"bash","args":{"command":"cinch impact; cinch --help"}}`,
		`{"type":"tool_execution_start","toolName":"read","args":{"path":"/work/.docs/workflows/docs-sync.md"}}`,
		`{"type":"tool_execution_start","toolName":"bash","args":{"command":"cat .docs/domain/money.md"}}`,
		`{"type":"tool_execution_start","toolName":"edit","args":{"path":"/work/.docs/domain/money.md"}}`,
		`{"type":"tool_execution_end","toolName":"bash","result":{"content":[{"type":"text","text":"commit error .git/COMMIT_EDITMSG:1: bad\ncinch: hook: pre-commit: 1 of 4 steps failed\n  checks      failed (exit status 1)\n  make-check  ok\n"}]}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u, err := scanTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if u.Cinch["check"] != 1 || u.Cinch["impact"] != 1 || u.Cinch["help"] != 1 || len(u.Cinch) != 3 {
		t.Errorf("cinch = %v", u.Cinch)
	}
	if !u.ReadsDocs || !u.ReadsFlow || !u.EditsDoc {
		t.Errorf("usage = %+v", u)
	}
	if u.Findings["commit error"] != 1 || u.Findings["checks failed (cinch hook step)"] != 1 || len(u.Findings) != 2 {
		t.Errorf("findings = %v", u.Findings)
	}
}
