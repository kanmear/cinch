// Command field turns the field data behind claim #7 into published tables.
//
//	field hooklog [-o DIR] LOG.tsv...
//	    Parses the hook logs of plan 19 (.git/cinch-log.tsv), classifies every
//	    row, and writes DIR/hook-log.csv and DIR/findings.csv. Prints the
//	    per-repo summary as a Markdown table.
//
//	field rulegrowth [-o DIR] [-name NAME] [-since DATE] REPO BRANCH
//	    Plan 20's walk: for each `merge: <name>` commit reachable from BRANCH,
//	    checks the commit and its first parent out in a detached worktree, runs
//	    `cinch rules --json`, and writes DIR/rule-growth-NAME.csv. Prints the
//	    headline: the share of feature/ and fix/ merges touching product code
//	    that added or changed a rule.
//
// Both read git data only. rulegrowth adds and removes temporary worktrees in
// REPO, so point it at a clone to leave the original untouched.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "hooklog":
		err = hooklogCmd(os.Args[2:])
	case "rulegrowth":
		err = rulegrowthCmd(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "field:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: field hooklog [-o DIR] LOG.tsv...\n       field rulegrowth [-o DIR] [-name NAME] [-since DATE] REPO BRANCH")
	os.Exit(2)
}
