package cinch

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentity_AbsentTombstoneKeyIsNoOp(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n"+
			"2. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "drop CIN-001")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if result.NoOp == "" {
		t.Fatalf("want a no-op (rules.tombstone unset — opt-in, not configured), got a real comparison")
	}
	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings on a no-op, got %+v", result.Findings)
	}
}

func TestIdentity_IDRemovedWithMatchingTombstoneIsClean(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "rules:\n  tombstone: '~~[A-Z0-9]+-[0-9]+~~'\n")
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n"+
			"2. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. ~~CIN-001~~ retired: superseded by CIN-002.\n"+
			"2. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "tombstone CIN-001")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings (tombstoned), got %+v", result.Findings)
	}
}

func TestIdentity_IDRemovedWithTombstoneSetButNoMarkerFires(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "rules:\n  tombstone: '~~[A-Z0-9]+-[0-9]+~~'\n")
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n"+
			"2. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "seed")

	// rules.tombstone is configured, but this removal doesn't use it —
	// the key being set doesn't waive the check, it names the convention.
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "drop CIN-001 without tombstoning")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 1 || !strings.HasPrefix(result.Findings[0].Message, "CIN-001") {
		t.Fatalf("want 1 finding for CIN-001, got %+v", result.Findings)
	}
}

func TestIdentity_NormalEditNoIDsTouchedIsClean(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "rules:\n  tombstone: '~~[A-Z0-9]+-[0-9]+~~'\n")
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n")
	gitCommitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text, reworded.\n")
	gitCommitAll(t, dir, "reword CIN-001")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
}

func TestIdentity_WholeFileDeletedFiresPerLostID(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "rules:\n  tombstone: '~~[A-Z0-9]+-[0-9]+~~'\n")
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n"+
			"2. **CIN-002** the second rule's text.\n")
	gitCommitAll(t, dir, "seed")

	runGit(t, dir, "rm", "-q", ".docs/rules.md")
	gitCommitAll(t, dir, "delete rules.md entirely")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 2 {
		t.Fatalf("want 2 findings (one per lost ID), got %+v", result.Findings)
	}
}

func TestIdentity_InvalidTombstoneRegexpFires(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, "cinch.yml"), "rules:\n  tombstone: '[unterminated'\n")
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"),
		"1. **CIN-001** the first rule's text.\n")
	gitCommitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".docs", "rules.md"), "no rules here.\n")
	gitCommitAll(t, dir, "drop CIN-001")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 1 || !strings.Contains(result.Findings[0].Message, "rules.tombstone is not a valid regexp") {
		t.Fatalf("want 1 finding naming the bad regexp, got %+v", result.Findings)
	}
}

func TestIdentity_NotAGitRepoAnnouncesNoOp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"), "1. **CIN-001** text.\n")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message for a non-git directory, got none")
	}
}

func TestIdentity_SingleCommitAnnouncesNoOp(t *testing.T) {
	dir := gitInitRepo(t)
	writeFile(t, filepath.Join(dir, ".docs", "rules.md"), "1. **CIN-001** text.\n")
	gitCommitAll(t, dir, "only commit")

	result := checkIdentity(filepath.Join(dir, ".docs"), dir)

	if len(result.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", result.Findings)
	}
	if result.NoOp == "" {
		t.Fatalf("want an explicit no-op message for a single-commit repo, got none")
	}
}
