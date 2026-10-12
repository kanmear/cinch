package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGrade(t *testing.T) {
	bound := []string{"m/tests/models.TestA"}
	cases := []struct {
		failing []string
		want    string
	}{
		{nil, "silent"},
		{[]string{"m/tests/models.TestB"}, "other"},
		{[]string{"m/tests/models.TestB", "m/tests/models.TestA"}, "marked"},
		{[]string{"m/tests/handlers (package)"}, "other"},
	}
	for _, c := range cases {
		if got := grade(c.failing, bound); got != c.want {
			t.Errorf("grade(%v) = %s, want %s", c.failing, got, c.want)
		}
	}
}

func TestFailures(t *testing.T) {
	out := []byte(`{"Action":"run","Package":"m/a","Test":"TestPass"}
{"Action":"pass","Package":"m/a","Test":"TestPass"}
{"Action":"run","Package":"m/a","Test":"TestSub"}
{"Action":"run","Package":"m/a","Test":"TestSub/case"}
{"Action":"fail","Package":"m/a","Test":"TestSub/case"}
{"Action":"fail","Package":"m/a","Test":"TestSub"}
{"Action":"fail","Package":"m/a"}
{"Action":"run","Package":"m/b","Test":"TestPanics"}
{"Action":"output","Package":"m/b","Test":"TestPanics","Output":"panic: boom\n"}
{"Action":"fail","Package":"m/b"}
{"Action":"fail","Package":"m/c"}
{"Action":"pass","Package":"m/d"}
`)
	tests, packages := failures(out)
	// A subtest's failure is its top-level test's; a test that started and
	// never finished was running when the binary died.
	if want := []string{"m/a.TestSub", "m/b.TestPanics"}; !reflect.DeepEqual(tests, want) {
		t.Errorf("tests = %v, want %v", tests, want)
	}
	// m/c failed with no test failing, e.g. a TestMain error.
	if want := []string{"m/c"}; !reflect.DeepEqual(packages, want) {
		t.Errorf("packages = %v, want %v", packages, want)
	}
}

func TestRuleTextDropsTrailingHeading(t *testing.T) {
	got := ruleText("7. **PERM-007** Lowering a role's permission is confirm-gated. ## Related Documentation - [x](y)")
	if want := "7. **PERM-007** Lowering a role's permission is confirm-gated."; got != want {
		t.Errorf("ruleText = %q, want %q", got, want)
	}
}

func TestTestDatabaseURL(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := "DATABASE_URL=postgres://u:p@localhost:5432/app?sslmode=disable\n" +
		"TEST_DATABASE_URL=postgres://u:p@localhost:5432/app_test?sslmode=disable\n"
	if err := os.WriteFile(filepath.Join(repo, "backend", ".env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := testDatabaseURL(config{repo: repo, db: "bench_db", pgPort: 55432})
	if err != nil {
		t.Fatal(err)
	}
	if want := "postgres://u:p@localhost:55432/bench_db?sslmode=disable"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFinishedIgnoresTestsThatNeverEnded(t *testing.T) {
	out := []byte(`{"Action":"run","Package":"m/a","Test":"TestDone"}
{"Action":"pass","Package":"m/a","Test":"TestDone"}
{"Action":"run","Package":"m/a","Test":"TestPanics"}
{"Action":"fail","Package":"m/a"}
`)
	got := finished(out)
	if !got["m/a.TestDone"] || got["m/a.TestPanics"] || got["m/a.TestNeverStarted"] {
		t.Errorf("finished = %v", got)
	}
}
