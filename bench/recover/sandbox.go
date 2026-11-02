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
	sandboxScript = "/fake/agent.sh"
)

// sandboxPaths are the host paths a sandbox binds in; resolveEngine fills them.
type sandboxPaths struct {
	home, flutter, pubCache string
	piNode, piCLI           string // node and pi's cli.js
	piModels                string // ~/.pi/agent/models.json
	fakeScript              string
}

// sandboxSpec says what one sandbox holds beyond the shared toolchain.
type sandboxSpec struct {
	repo     string // host path of the checkout, bound at /work
	rundir   string // holds the files generated for the sandbox's home
	engine   string // pi, fake or "" for tool-only (probes)
	readOnly bool   // bind the checkout read-only (the setup probe)
}

// sandboxArgs builds the bwrap arguments (not the command) for one agent
// process. Both arms get the same sandbox, byte for byte, and bind no
// directory that holds a cinch, whatever PATH a shell builds. The checkout
// appears at /work, so nothing names the run, the arm or the cache. Network
// stays shared: pi needs the lab endpoint.
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

	path := []string{"/usr/local/bin", "/usr/bin"}
	// Flutter writes into its own SDK directory (a cache lock, version stamps),
	// so the SDK and the pub cache are overlays: readable, writable per run,
	// and discarded at exit. Only when the fixture needs them.
	if p.flutter != "" {
		args = append(args, "--overlay-src", p.flutter, "--tmp-overlay", p.flutter)
		path = append([]string{filepath.Join(p.flutter, "bin"), filepath.Join(p.flutter, "bin", "cache", "dart-sdk", "bin")}, path...)
		if _, err := os.Stat(p.pubCache); err == nil {
			args = append(args, "--overlay-src", p.pubCache, "--tmp-overlay", p.pubCache)
		}
	}

	home := func(rel string) string { return filepath.Join(p.home, rel) }
	switch s.engine {
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
	// Proxies that arrive by environment rather than by file.
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasSuffix(upper, "_PROXY") || name == "SSL_CERT_FILE" || name == "NODE_EXTRA_CA_CERTS" {
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
	p.home = home
	p.pubCache = envOr("PUB_CACHE", filepath.Join(home, ".pub-cache"))
	if h.flutter != "" {
		if _, err := os.Stat(h.flutter); err == nil {
			p.flutter = h.flutter
		}
	}
	switch h.engine {
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

// verifySandbox checks the sandbox before any run: no cinch reachable,
// the toolchain present, and the user's code directory not visible.
func (h *harness) verifySandbox() error {
	scratch, err := os.MkdirTemp("", "recover-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	spec := sandboxSpec{repo: h.baseDir, rundir: scratch, readOnly: true}
	cmd, err := h.sandboxCmd(spec, "/bin/sh", "-c", `
		command -v git >/dev/null || echo "no git"
		command -v grep >/dev/null || echo "no grep"
		[ -e "$HOME/code" ] && echo "code is visible"
		command -v cinch || echo "no cinch"
		exit 0`)
	if err != nil {
		return err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sandbox: %v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "no cinch" {
		return fmt.Errorf("sandbox probe printed %q, want %q", got, "no cinch")
	}
	return nil
}
