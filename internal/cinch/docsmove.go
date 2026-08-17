package cinch

import (
	"os"
	"os/exec"
	"path/filepath"

	"cinch/internal/output"
)

// CmdMoveDocs implements `cinch move-docs NEW-PATH`: moves the docs root on
// disk (git mv in a git repo, plain rename otherwise), rewrites paths.docs
// in cinch.yml, and re-renders — as one operation, so the manifest never
// points at a path while the old directory still sits abandoned (the window
// checkGenerated's cross-root check exists to catch when this command isn't
// used).
func CmdMoveDocs(root, newPath string) int {
	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("move-docs", err)
	}
	oldPath := docsPathValue(m)

	if filepath.IsAbs(oldPath) || filepath.IsAbs(newPath) {
		return output.Failf("move-docs", "paths.docs must be repo-relative")
	}
	oldClean, newClean := filepath.Clean(oldPath), filepath.Clean(newPath)
	if oldClean == newClean {
		output.Step("paths.docs is already %s", oldClean)
		return 0
	}

	oldAbs, newAbs := filepath.Join(root, oldClean), filepath.Join(root, newClean)
	if _, err := os.Stat(newAbs); err == nil {
		return output.Failf("move-docs", "%s already exists — refusing to overwrite", newClean)
	}

	if _, err := os.Stat(oldAbs); err == nil {
		if isGitRepo(root) {
			cmd := exec.Command("git", "mv", oldClean, newClean)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				return output.Failf("move-docs", "git mv failed: %v\n%s", err, out)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
				return output.Fail("move-docs", err)
			}
			if err := os.Rename(oldAbs, newAbs); err != nil {
				return output.Fail("move-docs", err)
			}
		}
		output.Step("moved %s -> %s", oldClean, newClean)
	} else {
		output.Step("%s does not exist on disk — nothing to move, updating manifest only", oldClean)
	}

	if err := writeManifestValue(root, pathsDocsKey, newClean); err != nil {
		return output.Failf("move-docs", "rewriting manifest: %s", err.Error())
	}
	output.Step("cinch.yml: paths.docs = %s", newClean)

	if code := CmdRender(root); code != 0 {
		return code
	}
	if isGitRepo(root) {
		output.Step("next: `git add cinch.yml` (the directory move is already staged by git mv), then `cinch check` and commit")
	} else {
		output.Step("next: `cinch check` and commit")
	}
	return 0
}
