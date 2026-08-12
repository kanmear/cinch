package cinch

import (
	"path/filepath"
	"testing"
)

const testCommitPatternManifest = `commit:
  pattern: '^\[[a-z-]+\] .+'
`

func TestCheckCommit_MatchingPatternIsClean(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testCommitPatternManifest)
	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, msgFile, "[docs] fix a typo\n")

	if got := checkCommit(root, msgFile); len(got.Findings) != 0 {
		t.Fatalf("want 0 findings, got %+v", got)
	}
}

func TestCheckCommit_NonMatchingPatternFires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testCommitPatternManifest)
	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, msgFile, "fix a typo\n")

	got := checkCommit(root, msgFile)
	if len(got.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got.Findings), got)
	}
	if got.Findings[0].Check != "commit" || got.Findings[0].Level != "error" {
		t.Fatalf("unexpected finding: %+v", got.Findings[0])
	}
}

func TestCheckCommit_AbsentKeyIsNoBehaviorChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), "paths:\n  docs: .docs\n")
	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, msgFile, "anything goes here\n")

	if got := checkCommit(root, msgFile); len(got.Findings) != 0 {
		t.Fatalf("want 0 findings with no commit.pattern key, got %+v", got)
	}
}

func TestCheckCommit_NoMsgFileIsNoOp(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testCommitPatternManifest)

	got := checkCommit(root, "")
	if len(got.Findings) != 0 {
		t.Fatalf("want 0 findings with no msgFile, got %+v", got)
	}
	if got.NoOp == "" {
		t.Fatalf("want a no-op reason when no msgFile is given, got none")
	}
}

func TestCheckCommit_OnlySubjectLineMatched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cinch.yml"), testCommitPatternManifest)
	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	writeFile(t, msgFile, "[docs] fix a typo\n\nBody line not matching the pattern at all.\n")

	if got := checkCommit(root, msgFile); len(got.Findings) != 0 {
		t.Fatalf("want 0 findings — only the subject line should be checked, got %+v", got)
	}
}
