package cinch

import (
	"fmt"
	"path/filepath"
	"strings"

	"cinch/internal/gitutil"
)

func checkHooks(root string, m *manifest, mErr error) checkResult {
	if !gitutil.IsRepo(root) {
		return checkResult{noOp: "not a git repository"}
	}

	if mErr != nil || m == nil {
		return checkResult{noOp: "cinch.yml not found — hooks activation not checked"}
	}

	want := hooksPathValue(m)

	out, err := gitutil.Output(root, "config", "--get", "core.hooksPath")
	got := strings.TrimSpace(string(out))
	if err != nil || got == "" {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is not set — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", want)}
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		return checkResult{noOp: fmt.Sprintf("git core.hooksPath is %q, expected %q (paths.hooks) — hooks are not active; run 'cinch init' or 'git config core.hooksPath %s'", got, want, want)}
	}
	return checkResult{}
}
