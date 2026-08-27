package cinch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cinch/internal/output"
)

const (
	releaseRepo      = "kanmear/cinch"
	releaseAPIURL    = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
	installScriptURL = "https://raw.githubusercontent.com/" + releaseRepo + "/main/install.sh"
)

// maybeInstallLatest checks whether a newer cinch release exists and, if the
// user confirms, installs it (by shelling out to install.sh, the same
// audited curl+checksum+install logic the documented install method uses)
// and re-execs into it so the rest of CmdUpgrade runs against the version
// that was just installed.
//
// It returns handled=true only when CmdUpgrade should stop and return code
// immediately (an install was attempted and failed, or a fresh binary was
// installed and re-exec itself failed). In every other case — no newer
// release, network unreachable, windows, no bash, non-interactive, or the
// user declined — it returns handled=false so CmdUpgrade continues with the
// local render/sync/check flow.
// releaseCheckDisabled lets tests skip the network round-trip entirely;
// production code never sets this.
var releaseCheckDisabled = false

func maybeInstallLatest() (code int, handled bool) {
	if Version == "dev" || releaseCheckDisabled {
		return 0, false
	}
	if runtime.GOOS == "windows" {
		output.Skip("upgrade", fmt.Sprintf("self-install isn't supported on windows — download the latest release from https://github.com/%s/releases", releaseRepo))
		return 0, false
	}

	tag, ok := latestReleaseTag()
	if !ok {
		return 0, false
	}
	remote := strings.TrimPrefix(tag, "v")

	if !isNewerRelease(Version, remote) {
		output.Step("installed cinch %s is already the latest release", Version)
		return 0, false
	}

	if _, err := exec.LookPath("bash"); err != nil {
		output.Skip("upgrade", fmt.Sprintf("cinch %s is available but bash isn't on PATH — install manually: https://github.com/%s/releases/tag/%s", remote, releaseRepo, tag))
		return 0, false
	}

	if !output.IsInteractiveStdin(os.Stdin) {
		output.Step("cinch %s is available — run 'cinch upgrade' interactively to install it", remote)
		return 0, false
	}

	fmt.Printf("cinch %s is available (installed: %s) — install and continue? [y/N] ", remote, Version)
	if !promptYesNo() {
		output.Step("skipping install; continuing with local sync")
		return 0, false
	}

	execPath, err := os.Executable()
	if err != nil {
		return output.Fail("upgrade", err), true
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	installDir := filepath.Dir(execPath)

	if err := runInstallScript(tag, installDir); err != nil {
		return output.Fail("upgrade", err), true
	}

	output.Step("installed cinch %s -> %s; restarting to continue", remote, execPath)
	if err := reexec(execPath, os.Args, os.Environ()); err != nil {
		return output.Fail("upgrade", err), true
	}
	return 0, true // unreachable when reexec succeeds — it replaces this process
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// latestReleaseTag hits the GitHub releases API for the latest tag. Any
// failure (network, non-200, bad JSON) degrades gracefully: it reports why
// and returns ok=false rather than failing cinch upgrade outright, since the
// release check is best-effort on top of the local sync that already works.
func latestReleaseTag() (string, bool) {
	req, err := http.NewRequest(http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "cinch-upgrade")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		output.Skip("upgrade", fmt.Sprintf("could not reach GitHub to check for a newer release: %v", err))
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		output.Skip("upgrade", fmt.Sprintf("GitHub release check returned %s", resp.Status))
		return "", false
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.TagName == "" {
		output.Skip("upgrade", "could not parse the latest release info")
		return "", false
	}
	return release.TagName, true
}

// isNewerRelease reports whether remote is a strictly newer semver than
// installed. Unparseable versions (e.g. a hotfix build compared oddly, or a
// malformed tag) are treated as "not newer" rather than erroring — the
// caller falls back to the local-only flow either way.
func isNewerRelease(installed, remote string) bool {
	pi, oki := parseSemver(installed)
	pr, okr := parseSemver(remote)
	if !oki || !okr {
		return false
	}
	return semverLess(pi, pr)
}

// runInstallScript fetches the current install.sh from the repo and runs it
// with CINCH_VERSION pinned to tag and CINCH_INSTALL_DIR pointed at the
// currently running binary's directory, so the freshly installed binary
// lands at the exact path maybeInstallLatest re-execs into.
func runInstallScript(tag, installDir string) error {
	req, err := http.NewRequest(http.MethodGet, installScriptURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cinch-upgrade")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching install script: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching install script: %s", resp.Status)
	}
	script, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("fetching install script: %w", err)
	}

	tmp, err := os.CreateTemp("", "cinch-install-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(script); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	cmd := exec.Command("bash", tmp.Name())
	cmd.Env = append(os.Environ(), "CINCH_VERSION="+tag, "CINCH_INSTALL_DIR="+installDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func promptYesNo() bool {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}
