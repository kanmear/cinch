package tests

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellUsage(t *testing.T) {
	out, err := exec.Command("../bin/cinch").CombinedOutput()
	if err == nil {
		t.Fatalf("no-args run: want exit 2, got nil\n%s", out)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("no-args run: want exit code 2, got %v", err)
	}
	if !strings.Contains(string(out), "usage") {
		t.Fatalf("no-args run: usage not printed:\n%s", out)
	}
}

func TestUnknownCommandNamesPurge(t *testing.T) {
	out, err := exec.Command("../bin/cinch", "check").CombinedOutput()
	if err == nil {
		t.Fatalf("unknown command: want non-zero exit, got nil\n%s", out)
	}
	if !strings.Contains(string(out), "rebuild") {
		t.Fatalf("unknown command: rebuild notice not printed:\n%s", out)
	}
}
