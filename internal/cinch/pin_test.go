package cinch

import (
	"strings"
	"testing"
)

func TestCheckPin(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		r := checkPin(t.TempDir(), "1.0.0")
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("dev build skips the check", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.0.0\n")
		r := checkPin(directory, "dev")
		if r.noOp == "" || len(r.findings) != 0 {
			t.Fatalf("result = %+v, want noOp", r)
		}
	})

	t.Run("exact match", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.2.3\n")
		r := checkPin(directory, "1.2.3")
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("exact mismatch", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: 1.2.3\n")
		r := checkPin(directory, "1.2.4")
		if len(r.findings) != 1 || r.findings[0].check != "core" {
			t.Fatalf("result = %+v, want one core finding", r)
		}
	})

	t.Run("minimum satisfied", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=1.0.0'\n")
		r := checkPin(directory, "1.2.3")
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("minimum satisfied exactly", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=1.2.3'\n")
		r := checkPin(directory, "1.2.3")
		if len(r.findings) != 0 || r.noOp != "" {
			t.Fatalf("result = %+v, want ok", r)
		}
	})

	t.Run("minimum not satisfied", func(t *testing.T) {
		directory := t.TempDir()
		writeTestFile(t, directory, "cinch.yml", "require:\n  cinch: '>=2.0.0'\n")
		r := checkPin(directory, "1.9.9")
		if len(r.findings) != 1 || r.findings[0].check != "core" {
			t.Fatalf("result = %+v, want one core finding", r)
		}
		if want := "does not satisfy require.cinch >=2.0.0"; !strings.Contains(r.findings[0].message, want) {
			t.Fatalf("message = %q, want it to contain %q", r.findings[0].message, want)
		}
	})
}
