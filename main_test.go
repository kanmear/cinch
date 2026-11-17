package main

import (
	"strings"
	"testing"
)

func TestHelpTextListsEveryCommandAndFlag(t *testing.T) {
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if !strings.Contains(helpText, "  cinch "+command) {
				t.Fatalf("help text does not document %q", command)
			}
		})
	}
	for _, flag := range []string{"--changed", "--staged", "--only", "--links-to", "--links-from", "--json", "MSGFILE", "-h and --help"} {
		t.Run(flag, func(t *testing.T) {
			if !strings.Contains(helpText, flag) {
				t.Fatalf("help text does not document %q", flag)
			}
		})
	}
}

func TestHelpTextGroupsByTask(t *testing.T) {
	last := -1
	for _, heading := range []string{"\nsetup\n", "\ncheck\n", "\ninspect\n", "\nplumbing\n"} {
		at := strings.Index(helpText, heading)
		if at <= last {
			t.Fatalf("help text has heading %q at %d, want it after the previous group (%d)", strings.TrimSpace(heading), at, last)
		}
		last = at
	}
	if strings.Contains(helpText, "docs/templates") {
		t.Fatal("help text names the internal docs/templates path")
	}
}

func TestHelpTextListsEveryOnlyName(t *testing.T) {
	for name := range checkNames {
		if !strings.Contains(helpText, name) {
			t.Fatalf("help text does not list --only name %q", name)
		}
	}
}
