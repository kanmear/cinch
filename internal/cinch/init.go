package cinch

import (
	"os"
	"os/exec"
	"path/filepath"

	"cinch/internal/output"
)

const starterManifest = `paths:
  docs: .docs
  hooks: .githooks

# hooks:
#   pre-commit:
#     example:
#       run: make check
#       when: [src/]
`

func CmdInit(root string) int {
	manifestFile := filepath.Join(root, manifestPath)
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		if err := os.WriteFile(manifestFile, []byte(starterManifest), 0o644); err != nil {
			return output.Fail("init", err)
		}
		output.Step("wrote %s", manifestPath)
	} else {
		output.Step("%s already exists — left as-is", manifestPath)
	}

	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("init", err)
	}

	if code := CmdRender(root); code != 0 {
		return code
	}

	if isGitRepo(root) {
		hooksDir := hooksPathValue(m)
		cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return output.Failf("init", "git config core.hooksPath failed: %v\n%s", err, out)
		}
		output.Step("activated hooks: core.hooksPath = %s", hooksDir)
	} else {
		output.Skip("init", "hooks", "not a git repository — hook shims generated but not activated")
	}

	return 0
}
