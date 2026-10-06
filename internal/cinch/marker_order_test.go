package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// orderMarker is split so this file's own fixtures never read as markers
// when cinch scans its own repository.
const orderMarker = "// cinch:" + "rule "

// cinch:rule CINCH-010
func TestScanRuleMarkersOrderIsStable(t *testing.T) {
	root := t.TempDir()
	// Enough files that the worker pool's merge order would scramble them.
	for i := 0; i < 64; i++ {
		body := fmt.Sprintf("package x\n\n%sORD-1\n\n%sORD-1\n", orderMarker, orderMarker)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%02d.go", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for run := 0; run < 5; run++ {
		markers, _, err := scanRuleMarkers(workingTreeRoots(root), filepath.Join(root, "docs"), markerScanOptions{})
		if err != nil {
			t.Fatalf("scanRuleMarkers = %v, want nil", err)
		}
		locs := markers["ORD-1"]
		if len(locs) != 128 {
			t.Fatalf("markers[ORD-1] has %d locs, want 128", len(locs))
		}
		sorted := sort.SliceIsSorted(locs, func(i, j int) bool {
			if locs[i].file != locs[j].file {
				return locs[i].file < locs[j].file
			}
			return locs[i].line < locs[j].line
		})
		if !sorted {
			t.Fatalf("run %d: markers[ORD-1] not in file-then-line order: %v", run, locs[:8])
		}
	}
}
