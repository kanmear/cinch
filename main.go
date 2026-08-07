package main

import (
	"fmt"
	"os"
)

const usage = `cinch — referential integrity checker for the operational
documentation that governs a repository (rules, workflows, conventions)

usage:
  cinch check [-root DIR]   run the checks against the docs corpus
                            (default root: .docs)
  cinch                     print this usage (exit 2)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		os.Exit(cmdCheck(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "cinch: %q is not a command yet — the rebuild has not landed\n", os.Args[1])
		os.Exit(1)
	}
}
