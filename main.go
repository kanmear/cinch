package main

import (
	"fmt"
	"os"
)

const usage = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

This installation is a shell: the previous implementation was purged
and the rebuild has not landed. See .docs/PRINCIPLES.md for what the rebuild
must be.

usage:
  cinch       print this usage (exit 2)
`

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "cinch: %q is not a command yet — the rebuild has not landed\n", os.Args[1])
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, usage)
	os.Exit(2)
}
