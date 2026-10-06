package cinch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// binaryMarker is split so this file's own fixtures never read as markers
// when cinch scans its own repository.
const binaryMarker = "// cinch:" + "rule "

// cinch:rule CINCH-009
func TestScanRuleMarkersSkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A compiled binary keeps string literals, so marker-shaped text can sit
	// in one. A NUL in the first 8 KB is git's own binary test.
	write("server", "\x7fELF\x00\x00\n"+binaryMarker+"BIN-1\n")
	// A NUL past the first 8 KB doesn't make a file binary, the same as git.
	write("late.go", binaryMarker+"LATE-1\n"+strings.Repeat("x", 8<<10)+"\x00\n")
	write("text.go", "package x\n\n"+binaryMarker+"TXT-1\n")

	markers, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), markerScanOptions{})
	if err != nil {
		t.Fatalf("scanRuleMarkers = %v, want nil", err)
	}
	if len(markers["BIN-1"]) != 0 {
		t.Fatalf("BIN-1 sits in a binary file and must not count, got %v", markers["BIN-1"])
	}
	if len(markers["LATE-1"]) != 1 {
		t.Fatalf("LATE-1's file has no NUL in its first 8 KB and must be scanned, got %v", markers["LATE-1"])
	}
	if len(markers["TXT-1"]) != 1 {
		t.Fatalf("TXT-1 in a text file must be found, got %v", markers["TXT-1"])
	}
}
