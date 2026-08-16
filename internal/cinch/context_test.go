package cinch

import (
	"path/filepath"
	"strings"
	"testing"
)

// seedContextRepo builds a repo with a rendered workflow and two plans, one
// carrying a status line and one without.
func seedContextRepo(t *testing.T) string {
	t.Helper()
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "paths:\n  docs: .docs\n")
	writeFile(t, filepath.Join(dir, ".docs", workflowsSubdir, "dev-fix-bug.md"),
		"# Bug Fix Workflow\n\nProduce a structured bug fix plan.\n")
	writeFile(t, filepath.Join(dir, ".docs", plansSubdir, "alpha.md"),
		"# Alpha Plan\n\nStatus: **proposed** — not started.\n\n## Context\n\nWords.\n")
	writeFile(t, filepath.Join(dir, ".docs", plansSubdir, "beta.md"),
		"# Beta Plan\n\nNo status line here.\n")
	return dir
}

func TestContext_ReportsBranchPlansWorkflowsAndStaged(t *testing.T) {
	dir := seedContextRepo(t)
	gitCommitAll(t, dir, "seed")
	writeFile(t, filepath.Join(dir, "newfile.go"), "package main\n")
	runGit(t, dir, "add", "newfile.go")

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	for _, want := range []string{
		"on main",
		"2 plans under",
		"alpha.md — Alpha Plan",
		"Status: **proposed** — not started.",
		"beta.md — Beta Plan",
		"| Workflow | Trigger |",             // the table, via workflowsTable
		"Produce a structured bug fix plan.", // its trigger, not its title
		"[dev-fix-bug.md](dev-fix-bug.md)",
		"1 staged path",
		"newfile.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context report missing %q:\n%s", want, got)
		}
	}
}

// A plan with no status line prints its title and nothing else — the status
// is reported verbatim when present, never invented or defaulted.
func TestContext_PlanWithoutStatusPrintsTitleOnly(t *testing.T) {
	dir := seedContextRepo(t)

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	beta := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "beta.md") {
			beta = line
		}
	}
	if beta != "beta.md — Beta Plan" {
		t.Fatalf("want beta.md's line to carry the title alone, got %q", beta)
	}
	if strings.Count(got, planStatusPrefix) != 1 {
		t.Fatalf("want exactly one status line (alpha's), got:\n%s", got)
	}
}

// The status line is printed verbatim rather than parsed into a vocabulary:
// a plan may say anything about itself, and context reports it unchanged.
func TestContext_StatusIsVerbatim(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, ".docs", plansSubdir, "odd.md"),
		"# Odd Plan\n\nStatus: half-done, blocked on a decision nobody has made.\n")

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	if !strings.Contains(got, "Status: half-done, blocked on a decision nobody has made.") {
		t.Fatalf("want the status line verbatim, got:\n%s", got)
	}
}

// A "Status:" deep in a plan's prose is not the plan's status.
func TestContext_StatusOnlyReadFromTheHead(t *testing.T) {
	dir := gitInitRepo(t)
	body := "# Deep Plan\n\n## Context\n\n" + strings.Repeat("filler line\n", 20) +
		"\nStatus: this is prose, not the plan's status.\n"
	writeFile(t, filepath.Join(dir, ".docs", plansSubdir, "deep.md"), body)

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	if strings.Contains(got, "this is prose") {
		t.Fatalf("a Status: buried in prose should not be read as the plan's status:\n%s", got)
	}
}

func TestContext_PlansAreFoundInSubdirectories(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, ".docs", plansSubdir, "fix", "bug.md"),
		"# A Bug Fix\n\nStatus: **proposed**.\n")

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	if !strings.Contains(got, "fix/bug.md — A Bug Fix") {
		t.Fatalf("want a nested plan listed with its relative path, got:\n%s", got)
	}
}

// Every missing input degrades to a stated skip and the rest still prints —
// a command meant to be the first thing a session runs must not refuse to
// run because one input is absent.
func TestContext_MissingInputsDegradeRatherThanFail(t *testing.T) {
	dir := t.TempDir() // not a git repo, no plans, no render
	writeFile(t, filepath.Join(dir, "cinch.yml"), "paths:\n  docs: .docs\n")

	got := contextReport(dir, filepath.Join(dir, ".docs"))

	if !strings.Contains(got, "no plans under") {
		t.Fatalf("want an explicit no-plans line, got:\n%s", got)
	}
	if code := CmdContext(dir); code != 0 {
		t.Fatalf("CmdContext with nothing to report: want exit 0, got %d", code)
	}
}

// Not a check: an unresolvable docs root is the only failure, since that is
// the one input context cannot report around.
func TestContext_UnresolvableDocsRootFails(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "foo: [unterminated\n")

	if code := CmdContext(dir); code == 0 {
		t.Fatalf("CmdContext with a malformed manifest: want non-zero, got 0")
	}
}
