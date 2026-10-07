package cinch

import (
	"path/filepath"
	"sync"
	"testing"
)

// The rules check and the impact advisory both need every marker. The hook
// must scan each file once and share the result: the scan is nearly all of
// the hook's cost, so a second pass doubles it.
func TestPreCommitHookScansEachFileOnce(t *testing.T) {
	root := t.TempDir()
	git := stagedTestRepo(t, root)
	writeTestFile(t, filepath.Join(root, "internal"), "auth.go", stagedMarkedCode+"// staged edit\n")
	git("add", "internal/auth.go")

	var mu sync.Mutex
	scans := map[string]int{}
	original := scanFileMarkersFunc
	scanFileMarkersFunc = func(path string) (map[string][]markerLoc, error) {
		mu.Lock()
		scans[filepath.Base(path)]++
		mu.Unlock()
		return original(path)
	}
	t.Cleanup(func() { scanFileMarkersFunc = original })

	if code := cmdHookPreCommit(root); code != 0 {
		t.Fatalf("cmdHookPreCommit = %d, want 0", code)
	}
	if scans["auth.go"] == 0 {
		t.Fatalf("scans = %v, want auth.go scanned", scans)
	}
	for file, n := range scans {
		if n != 1 {
			t.Errorf("%s scanned %d times in one hook run, want 1 (scans = %v)", file, n, scans)
		}
	}
}
