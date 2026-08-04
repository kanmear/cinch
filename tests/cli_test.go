// Package tests exercises the built cinch binary end-to-end — the harness
// guarding its own repo (D036). `make test` builds bin/cinch before testing.
package tests

import (
	"os/exec"
	"regexp"
	"testing"
)

var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func TestVersionShow(t *testing.T) {
	out, err := exec.Command("../bin/cinch", "version", "show").Output()
	if err != nil {
		t.Fatalf("cinch version show: %v", err)
	}
	if !versionRe.Match(out) {
		t.Fatalf("cinch version show = %q, want x.y.z", out)
	}
}

func TestSelfCheck(t *testing.T) {
	out, err := exec.Command("../bin/cinch", "check").CombinedOutput()
	if err != nil {
		t.Fatalf("cinch check on its own repo failed: %v\n%s", err, out)
	}
}
