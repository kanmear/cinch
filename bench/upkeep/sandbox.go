package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	bwrapPath     = "/usr/bin/bwrap"
	sandboxWork   = "/work"
	sandboxCinch  = "/opt/cinch/bin"
	sandboxScript = "/fake/agent.sh"
)

// sandboxPaths are the host paths a sandbox binds in; setup resolves them.
type sandboxPaths struct {
	home, flutter, pubCache string
	claudeBin               string // claude's real binary
	credentials             string // ~/.claude/.credentials.json
	piNode, piCLI           string // node and pi's cli.js
	piModels                string // ~/.pi/agent/models.json
	fakeScript              string
}

// sandboxSpec says what one sandbox holds beyond the shared toolchain.
type sandboxSpec struct {
	arm      int
	repo     string // host path of the checkout, bound read-write at /work
	rundir   string // holds the files generated for the sandbox's home
	engine   string // pi, claude, fake or "" for tool-only (apply, probes)
	readOnly bool   // bind the checkout read-only (used by the setup probe)
}

// sandboxArgs builds the bwrap arguments (not the command) for one agent
// process. Every arm gets the same sandbox, byte for byte, except that arm 3
// binds the grader's cinch at one fixed path on PATH; arms 1 and 2 bind no
// directory that holds a cinch at all, whatever PATH a shell builds. The
// checkout appears at /work, so nothing names the run, the arm or the cache.
// Network stays shared: the engines need the lab endpoint and the API.
func (h *harness) sandboxArgs(s sandboxSpec) ([]string, error) {
	p := h.sandbox
	args := []string{
		"--die-with-parent", "--new-session", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--clearenv",
		"--ro-bind", "/usr", "/usr",
		"--symlink", "usr/bin", "/bin", "--symlink", "usr/bin", "/sbin",
		"--symlink", "usr/lib", "/lib", "--symlink", "usr/lib", "/lib64",
		"--ro-bind", "/etc", "/etc",
		"--ro-bind-try", "/run/systemd/resolve", "/run/systemd/resolve", // /etc/resolv.conf points here
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--tmpfs", "/var/tmp",
		"--tmpfs", "/home", "--dir", p.home,
	}
	bindWork := "--bind"
	if s.readOnly {
		bindWork = "--ro-bind"
	}
	args = append(args, bindWork, s.repo, sandboxWork)
	// Flutter writes into its own SDK directory (a cache lock, version stamps), so
	// the SDK and the pub cache are overlays: readable, writable per run, and
	// discarded at exit. Nothing an agent does reaches the host or another run.
	args = append(args, "--overlay-src", p.flutter, "--tmp-overlay", p.flutter)
	if _, err := os.Stat(p.pubCache); err == nil {
		args = append(args, "--overlay-src", p.pubCache, "--tmp-overlay", p.pubCache)
	}

	path := []string{filepath.Join(p.flutter, "bin"), filepath.Join(p.flutter, "bin", "cache", "dart-sdk", "bin"), "/usr/local/bin", "/usr/bin"}
	if s.arm == 3 {
		args = append(args, "--ro-bind", h.cinchBin, sandboxCinch+"/cinch")
		path = append([]string{sandboxCinch}, path...)
	}

	home := func(rel string) string { return filepath.Join(p.home, rel) }
	switch s.engine {
	case "claude", "fake":
		// Only the credentials: none of the user's CLAUDE.md, memory, skills or
		// plugins. ~/.claude.json is a minimal file the engine may rewrite.
		state := filepath.Join(s.rundir, "claude.json")
		if err := os.WriteFile(state, []byte(`{"hasCompletedOnboarding":true}`+"\n"), 0o600); err != nil {
			return nil, err
		}
		args = append(args, "--dir", home(".claude"), "--bind", state, home(".claude.json"))
		if s.engine == "claude" {
			args = append(args, "--ro-bind", p.credentials, home(".claude/.credentials.json"), "--ro-bind", p.claudeBin, p.claudeBin)
		}
	case "pi":
		settings := filepath.Join(s.rundir, "pi-settings.json")
		minimal := fmt.Sprintf(`{"defaultProvider":%q,"defaultModel":%q,"defaultThinkingLevel":%q}`+"\n", h.provider, h.model, h.thinking)
		if err := os.WriteFile(settings, []byte(minimal), 0o644); err != nil {
			return nil, err
		}
		nodeRoot := filepath.Dir(filepath.Dir(p.piNode))
		packageRoot := p.piCLI // <npx dir>/node_modules/@scope/pkg/dist/bundle/cli.js, bound as <npx dir>
		for i := 0; i < 6; i++ {
			packageRoot = filepath.Dir(packageRoot)
		}
		args = append(args, "--dir", home(".pi/agent"),
			"--ro-bind", p.piModels, home(".pi/agent/models.json"),
			"--ro-bind", settings, home(".pi/agent/settings.json"),
			"--ro-bind", nodeRoot, nodeRoot, "--ro-bind", packageRoot, packageRoot)
		path = append(path, filepath.Dir(p.piNode))
	}
	if s.engine == "fake" && p.fakeScript != "" {
		args = append(args, "--ro-bind", p.fakeScript, sandboxScript)
	}

	env := map[string]string{
		"PATH": strings.Join(path, ":"), "HOME": p.home, "USER": os.Getenv("USER"), "LOGNAME": os.Getenv("USER"),
		"SHELL": "/bin/bash", "TERM": "dumb", "TMPDIR": "/tmp", "LANG": envOr("LANG", "C.UTF-8"),
		"PUB_CACHE": p.pubCache, "LD_LIBRARY_PATH": sandboxWork + "/.objectbox/lib",
		"BOT": "true", "CI": "true", "DART_SUPPRESS_ANALYTICS": "true", "FLUTTER_SUPPRESS_ANALYTICS": "true",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
	}
	if s.engine == "claude" {
		env["CLAUDE_CODE_DISABLE_AUTO_MEMORY"] = "1"
		env["DISABLE_AUTOUPDATER"] = "1"
	}
	// Credentials and proxies that arrive by environment rather than by file.
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(name, "ANTHROPIC_") || name == "CLAUDE_CODE_OAUTH_TOKEN" || strings.HasSuffix(upper, "_PROXY") ||
			name == "SSL_CERT_FILE" || name == "NODE_EXTRA_CA_CERTS" {
			env[name] = value
		}
	}
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		args = append(args, "--setenv", name, env[name])
	}
	return append(args, "--chdir", sandboxWork), nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// sandboxCmd wraps argv in the sandbox for s.
func (h *harness) sandboxCmd(s sandboxSpec, argv ...string) (*exec.Cmd, error) {
	args, err := h.sandboxArgs(s)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bwrapPath, append(append(args, "--"), argv...)...)
	cmd.Env = []string{"PATH=/usr/bin"}
	return cmd, nil
}

// resolveEngine finds what the chosen engine needs bound in.
func (h *harness) resolveEngine() error {
	if _, err := os.Stat(bwrapPath); err != nil {
		return fmt.Errorf("bubblewrap: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p := &h.sandbox
	p.home, p.flutter = home, h.flutter
	p.pubCache = envOr("PUB_CACHE", filepath.Join(home, ".pub-cache"))
	switch h.engine {
	case "claude":
		bin, err := exec.LookPath("claude")
		if err != nil {
			return fmt.Errorf("claude: %w", err)
		}
		if p.claudeBin, err = filepath.EvalSymlinks(bin); err != nil {
			return err
		}
		p.credentials = filepath.Join(home, ".claude", ".credentials.json")
		if _, err := os.Stat(p.credentials); err != nil {
			return fmt.Errorf("claude credentials: %w", err)
		}
	case "pi":
		if p.piNode, p.piCLI, err = findPi(home, h.piNode, h.piCLI); err != nil {
			return err
		}
		p.piModels = filepath.Join(home, ".pi", "agent", "models.json")
		if _, err := os.Stat(p.piModels); err != nil {
			return fmt.Errorf("pi models: %w", err)
		}
	case "fake":
		if h.fakeScript == "" {
			return fmt.Errorf("-engine fake wants -fake-script")
		}
		if p.fakeScript, err = filepath.Abs(h.fakeScript); err != nil {
			return err
		}
	}
	return nil
}

// findPi locates pi's node runtime and its cli.js without running the pi
// wrapper, which asks the network for updates.
func findPi(home, nodeFlag, cliFlag string) (node, cli string, err error) {
	node = nodeFlag
	if node == "" {
		if out, mErr := exec.Command("mise", "where", "node@latest").Output(); mErr == nil {
			node = filepath.Join(strings.TrimSpace(string(out)), "bin", "node")
		} else if node, err = exec.LookPath("node"); err != nil {
			return "", "", fmt.Errorf("pi needs node: pass -pi-node")
		}
	}
	if node, err = filepath.EvalSymlinks(node); err != nil {
		return "", "", err
	}
	cli = cliFlag
	if cli == "" {
		matches, _ := filepath.Glob(filepath.Join(home, ".npm", "_npx", "*", "node_modules", "@earendil-works", "pi-coding-agent", "dist", "bundle", "cli.js"))
		if len(matches) == 0 {
			return "", "", fmt.Errorf("pi's cli.js not found under ~/.npm/_npx: pass -pi-cli")
		}
		sort.Strings(matches)
		cli = matches[len(matches)-1]
	}
	cli, err = filepath.EvalSymlinks(cli)
	return node, cli, err
}

// verifySandboxes checks each arm's sandbox before any run: no cinch in arms
// 1 and 2, the grader's in arm 3, and the toolchain and the user's code
// directory as expected.
func (h *harness) verifySandboxes() error {
	scratch, err := os.MkdirTemp("", "upkeep-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	for _, arm := range canonicalArms {
		spec := sandboxSpec{arm: arm, repo: h.baseDir, rundir: scratch, readOnly: true}
		cmd, err := h.sandboxCmd(spec, "/bin/sh", "-c", `
			command -v flutter >/dev/null || echo "no flutter"
			command -v git >/dev/null || echo "no git"
			command -v make >/dev/null || echo "no make"
			[ -e "$HOME/code" ] && echo "code is visible"
			command -v cinch || echo "no cinch"
			exit 0`)
		if err != nil {
			return err
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("arm %d sandbox: %v: %s", arm, err, out)
		}
		got := strings.TrimSpace(string(out))
		want := "no cinch"
		if arm == 3 {
			want = sandboxCinch + "/cinch"
		}
		if got != want {
			return fmt.Errorf("arm %d sandbox probe printed %q, want %q", arm, got, want)
		}
	}
	return nil
}
