package main

import (
	"bytes"
	"math"
	"os"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestStats(t *testing.T) {
	one := []stratum{{Task: "A", XA: 2, NA: 2, XB: 0, NB: 2}}
	z, p, zCC, pCC := cmh(one)
	if !near(z, math.Sqrt(3)) || !near(p, 0.0416) || !near(zCC, 0.866) || !near(pCC, 0.1932) {
		t.Errorf("cmh = %v %v %v %v", z, p, zCC, pCC)
	}
	if got := exact(one); !near(got, 1.0/6) {
		t.Errorf("exact(one) = %v, want 1/6", got)
	}
	two := []stratum{one[0], {Task: "B", XA: 2, NA: 2, XB: 0, NB: 2}}
	if got := exact(two); !near(got, 1.0/36) {
		t.Errorf("exact(two) = %v, want 1/36", got)
	}
	flat := append(two, stratum{Task: "C", XA: 5, NA: 5, XB: 5, NB: 5})
	if got := exact(flat); !near(got, 1.0/36) {
		t.Errorf("exact(flat) = %v, want 1/36", got)
	}
	if got := diffPoints(flat); !near(got, 100*(9.0/9-5.0/9)) {
		t.Errorf("diffPoints = %v", got)
	}
	if got := median([]float64{5, 1, 3, 2}); got != 2.5 {
		t.Errorf("median = %v", got)
	}
	// 5 of 8 against 1 of 8 is Fisher's one-sided exact 476/8008.
	if got := exact([]stratum{{XA: 5, NA: 8, XB: 1, NB: 8}}); !near(got, 476.0/8008) {
		t.Errorf("fisher(5/8, 1/8) = %v, want 476/8008", got)
	}
}

func TestCapRounding(t *testing.T) {
	for in, want := range map[float64]int{0: 30, 10: 30, 20: 30, 21: 35, 41: 65, 49: 75, 50: 75, 120: 75} {
		if got := capFor(in); got != want {
			t.Errorf("capFor(%v) = %d, want %d", in, got, want)
		}
	}
}

func TestFinalAttemptAndInvalidHandling(t *testing.T) {
	runs, err := readRuns("testdata/results-fixture.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 79 {
		t.Fatalf("%d runs, want 79 (one run_id per run, the final attempt)", len(runs))
	}
	var t1r0, t2r7 *run
	for i := range runs {
		switch runs[i].ID {
		case "T1-arm1-r0":
			t1r0 = &runs[i]
		case "T2-arm1-r7":
			t2r7 = &runs[i]
		}
	}
	if t1r0 == nil || t1r0.Attempt != 1 || t1r0.Status == "invalid" {
		t.Errorf("the rerun attempt should replace the invalid first: %+v", t1r0)
	}
	if t2r7 == nil || t2r7.Status != "invalid" {
		t.Errorf("T2-arm1-r7: %+v", t2r7)
	}
	// An invalid final attempt is in no stratum.
	st := strataFor(runs, []string{"T2"}, 3, 1, isSafe)
	if len(st) != 1 || st[0].NB != 7 || st[0].XB != 2 || st[0].NA != 8 || st[0].XA != 4 {
		t.Errorf("T2 stratum %+v", st)
	}
}

func TestFixtureReport(t *testing.T) {
	runs, err := readRuns("testdata/results-fixture.csv")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	report(&out, "fixture", runs, 1)
	got := out.String()
	for _, want := range []string{
		"T1    1   8    7/8       5/7       2/8       1/8",
		"T2    1   7    6/7       4/6       2/7       1/7          0/7         0/7          0/7    (1 invalid, not counted)",
		"T2    3   8    6/8       2/6       4/8       2/8          3/8         2/7 +1p",
		"arm 3: 12/24 (50.0%)   arm 1: 5/23 (21.7%)   pooled difference: +28.3 points",
		"one-sided CMH:                   z = +2.017  p = 0.0218",
		"one-sided exact (stratified):               p = 0.0409",
		"look 1 (plain CMH p < 0.005 and pooled difference >= 15 points): not detected at this N",
		"T1 (in the step's own example) arm 3: 6/8 (75.0%)   arm 1: 2/8 (25.0%)",
		"T2-T3 pooled (out of the example) arm 3: 6/16 (37.5%)   arm 1: 3/15 (20.0%)",
		"T4   arm 1 leaked 2/7",
		"T1   arm 2 safe 4/5, leaked 0/4",
		"cap-bound tasks (more than 25% timed out in arm 1 or arm 3): none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	// Look 2 of the same data passes its (looser) threshold and says it is under-sized.
	out.Reset()
	report(&out, "fixture", runs, 2)
	if !strings.Contains(out.String(), "arm 3 beats arm 1") || !strings.Contains(out.String(), "registered at 15 per cell") {
		t.Errorf("look 2:\n%s", out.String())
	}
	if want, err := os.ReadFile("testdata/report-look1.golden"); err == nil {
		out.Reset()
		report(&out, "bench/leak/analyze/testdata/results-fixture.csv", runs, 1)
		if out.String() != string(want) {
			t.Errorf("report differs from testdata/report-look1.golden")
		}
	}
}

func TestCapBoundFlag(t *testing.T) {
	runs, _ := readRuns("testdata/results-fixture.csv")
	var out bytes.Buffer
	capBound(&out, runs, primaryTasks)
	if !strings.Contains(out.String(), "none") {
		t.Errorf("fixture is not cap-bound: %s", out.String())
	}
	// Timeouts above 25% in arm 3 of T3 flag it.
	for i := range runs {
		if runs[i].Task == "T3" && runs[i].Arm == 3 && runs[i].ID <= "T3-arm3-r3" {
			runs[i].TimedOut = true
		}
	}
	out.Reset()
	capBound(&out, runs, primaryTasks)
	if !strings.Contains(out.String(), "cap-bound tasks (more than 25% timed out in arm 1 or arm 3): T3") || !strings.Contains(out.String(), "without them") {
		t.Errorf("cap-bound output:\n%s", out.String())
	}
}

func TestPilotFixture(t *testing.T) {
	runs, err := readRuns("testdata/pilot-fixture.csv")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	reportPilot(&out, "pilot", runs)
	got := out.String()
	for _, want := range []string{
		"T1: KEEP, cap 65 minutes",
		"T2: not kept: (b) fails: 1 of 3 arm-1 pilots committed",
		"T3: not kept: (d) fails: all 3 arm-1 pilots are safe deliveries",
		"T4: not kept: (c) the arm-3 pilot did not commit: run one more arm-3 pilot",
		"KEPT_TASKS=T1\nCAP_T1=65\n",
		"fewer than two of T1-T3 survive (1)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pilot report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "CAP_T2") || strings.Contains(got, "CAP_T3=") {
		t.Errorf("caps are printed for tasks that are not kept:\n%s", got)
	}
	if want, err := os.ReadFile("testdata/report-pilot.golden"); err == nil {
		out.Reset()
		reportPilot(&out, "bench/leak/analyze/testdata/pilot-fixture.csv", runs)
		if out.String() != string(want) {
			t.Errorf("pilot report differs from testdata/report-pilot.golden")
		}
	}
	// A second arm-3 pilot that also fails drops the task.
	v := evalPilot("T4", append(runs, run{ID: "T4-arm3-r1", Task: "T4", Arm: 3, Pilot: true, Status: "graded"}))
	if !v.Dropped || v.NeedSecondArm3 {
		t.Errorf("T4 after a second failing arm-3 pilot: %+v", v)
	}
	// A second one that commits keeps (c).
	v = evalPilot("T4", append(runs, run{ID: "T4-arm3-r1", Task: "T4", Arm: 3, Pilot: true, Status: "graded", Committed: true, WallMin: 20}))
	if v.Dropped || v.NeedSecondArm3 {
		t.Errorf("T4 after a committing second arm-3 pilot: %+v", v)
	}
}

func TestPilotRowsAreSeparatedFromStudyRows(t *testing.T) {
	runs, _ := readRuns("testdata/pilot-fixture.csv")
	for _, r := range runs {
		if !r.Pilot {
			t.Fatalf("%s is not a pilot row", r.ID)
		}
	}
}
