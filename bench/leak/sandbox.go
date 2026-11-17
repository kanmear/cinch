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
	sandboxShim   = "/opt/shim/bin"
	sandboxBun    = "/opt/bun/bin"
	sandboxGoCash = "/cache/go-build"
	sandboxScript = "/fake/agent.sh"
	sandboxTrace  = "/run/git-trace2.jsonl"
	traceFileName = "git-trace2.jsonl"
)

// sandboxPaths are the host paths a sandbox binds in; setup resolves them.
type sandboxPaths struct {
	home          string
	goRoot        string // GOROOT, bound read-only (its bin/ also holds the toolchain's go and gofmt)
	goModCache    string // host module cache, the lower layer of a per-run overlay
	goBuildCache  string // harness-owned warm build cache, the lower layer of a per-run overlay
	bun           string // the real bun binary
	shimBin       string // directory holding the docker and docker-compose shims
	claudeBin     string // claude's real binary
	credentials   string // ~/.claude/.credentials.json
	piNode, piCLI string // node and pi's cli.js
	piModels      string // ~/.pi/agent/models.json
	fakeScript    string
}

// runEnv is what differs per run: the work database the sandbox is told about
// and the frontend dependencies bound under it.
type runEnv struct {
	dbURL       string // TEST_DATABASE_URL and DATABASE_URL inside the sandbox ("" for tool-only probes)
	nodeModules string // host dir of an installed frontend/node_modules, bound as an overlay ("" for none)
}

// sandboxSpec says what one sandbox holds beyond the shared toolchain.
type sandboxSpec struct {
	repo     string // host path of the checkout, bound read-write at /work
	rundir   string // holds the files generated for the sandbox's home
	engine   string // pi, claude, fake or "" for tool-only (apply, probes)
	readOnly bool   // bind the checkout read-only (used by the setup probe)
	env      runEnv
}

// sandboxArgs builds the bwrap arguments (not the command) for one agent
// process. Every arm gets the same sandbox, byte for byte: arms differ in the
// checkout and the prompt only. The checkout appears at /work, so nothing names
// the run, the arm or the cache. Network stays shared: the engines need the lab
// endpoint and the API, and the work database is a TCP port on the host.
//
// Docker is unreachable. There is no socket (only /usr and /etc are bound from
// the host), and the docker and docker-compose shims are first on PATH and
// also bound over the real binaries in /usr/bin, so no path reaches them.
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
	// The shims over the real binaries, so a command by absolute path is
	// refused too. Only where the host has them (the bind needs a mount point).
	for _, name := range shimNames {
		bound := map[string]bool{}
		for _, dir := range []string{"/usr/bin", "/usr/local/bin"} {
			// A symlink (docker-compose is one on some hosts) can't be a mount
			// point: bind over what it resolves to, which is where the binary is.
			dest, err := filepath.EvalSymlinks(filepath.Join(dir, name))
			if err != nil || !strings.HasPrefix(dest, "/usr/") || bound[dest] {
				continue
			}
			bound[dest] = true
			args = append(args, "--ro-bind", filepath.Join(p.shimBin, name), dest)
		}
	}
	args = append(args, "--ro-bind", p.shimBin, sandboxShim)
	// docker's cli plugins are reached through the docker binary, which is the shim.

	bindWork := "--bind"
	if s.readOnly {
		bindWork = "--ro-bind"
	}
	args = append(args, bindWork, s.repo, sandboxWork)

	// Toolchain: Go read-only at its own path; the module cache and the build
	// cache are overlays, readable, writable per run, discarded at exit. Nothing
	// an agent does reaches the host or another run.
	args = append(args, "--ro-bind", p.goRoot, p.goRoot)
	gopath := filepath.Join(p.home, "go")
	args = append(args, "--dir", gopath)
	if _, err := os.Stat(p.goModCache); err == nil {
		args = append(args, "--overlay-src", p.goModCache, "--tmp-overlay", filepath.Join(gopath, "pkg", "mod"))
	}
	if _, err := os.Stat(p.goBuildCache); err == nil {
		args = append(args, "--overlay-src", p.goBuildCache, "--tmp-overlay", sandboxGoCash)
	}
	args = append(args, "--ro-bind", p.bun, sandboxBun+"/bun")
	if s.env.nodeModules != "" {
		args = append(args, "--overlay-src", s.env.nodeModules, "--tmp-overlay", sandboxWork+"/frontend/node_modules")
	}
	args = append(args, "--ro-bind", h.cinchBin, sandboxCinch+"/cinch")

	path := []string{sandboxShim, sandboxCinch, filepath.Join(p.goRoot, "bin"), sandboxBun, "/usr/local/bin", "/usr/bin"}

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
		"GOPATH": gopath, "GOCACHE": sandboxGoCash, "GOFLAGS": "", "GOTOOLCHAIN": "local",
		"BUN_INSTALL_CACHE_DIR": "/tmp/bun-cache",
		"CI":                    "true", "BOT": "true",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
	}
	// git's own trace of every hook it runs, written outside the repository, so
	// hook runs and landed commits are countable whether or not the state has a
	// field-log script (T1's and T4's bases do not). The agent can't see it in
	// the tree; grading reads it (hookTrace).
	if s.rundir != "" && !s.readOnly {
		trace := filepath.Join(s.rundir, traceFileName)
		f, err := os.OpenFile(trace, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		f.Close()
		args = append(args, "--bind", trace, sandboxTrace)
		env["GIT_TRACE2_EVENT"] = sandboxTrace
		env["GIT_TRACE2_EVENT_NESTING"] = "10"
	}
	if s.env.dbURL != "" {
		env["TEST_DATABASE_URL"] = s.env.dbURL
		env["DATABASE_URL"] = s.env.dbURL
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

// resolveToolchain finds Go, bun, the caches and the shims, whatever the engine.
func (h *harness) resolveToolchain() error {
	if _, err := os.Stat(bwrapPath); err != nil {
		return fmt.Errorf("bubblewrap: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p := &h.sandbox
	p.home = home
	out, err := exec.Command("go", "env", "GOROOT", "GOMODCACHE").Output()
	if err != nil {
		return fmt.Errorf("go env: %w", err)
	}
	fields := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(fields) != 2 {
		return fmt.Errorf("go env printed %q", out)
	}
	if p.goRoot, err = filepath.EvalSymlinks(fields[0]); err != nil {
		return err
	}
	p.goModCache = fields[1]
	p.goBuildCache = filepath.Join(h.work, "gocache")
	if err := os.MkdirAll(p.goBuildCache, 0o755); err != nil {
		return err
	}
	if p.bun, err = findBun(home); err != nil {
		return err
	}
	if p.shimBin, err = writeShims(filepath.Join(h.work, "shim"), h.pg.user); err != nil {
		return err
	}
	return nil
}

// findBun locates the real bun binary (not a shim script).
func findBun(home string) (string, error) {
	candidates := []string{filepath.Join(home, ".bun", "bin", "bun")}
	if path, err := exec.LookPath("bun"); err == nil {
		candidates = append(candidates, path)
	}
	for _, c := range candidates {
		if real, err := filepath.EvalSymlinks(c); err == nil {
			if st, err := os.Stat(real); err == nil && !st.IsDir() {
				return real, nil
			}
		}
	}
	return "", fmt.Errorf("bun not found: install it or put it on PATH")
}

// resolveEngine finds what the chosen engine needs bound in.
func (h *harness) resolveEngine() error {
	home := h.sandbox.home
	p := &h.sandbox
	var err error
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

// probeScript is the sandbox check run before any scheduled run (and the shape
// of the smoke prompt's fixed probes): the toolchain present, cinch the pinned
// one, docker unreachable by name and by path, the user's code directory
// absent.
const probeScript = `
command -v go >/dev/null || echo "no go"
command -v bun >/dev/null || echo "no bun"
command -v git >/dev/null || echo "no git"
command -v make >/dev/null || echo "no make"
[ -e "$HOME/code" ] && echo "code is visible"
echo "cinch: $(command -v cinch)"
echo "docker: $(command -v docker)"
docker ps >/dev/null 2>&1 && echo "docker ps worked"
/usr/bin/docker ps >/dev/null 2>&1 && echo "/usr/bin/docker ps worked"
[ -S /var/run/docker.sock ] && echo "docker socket is visible"
exit 0`

// verifySandbox checks the sandbox before any run.
func (h *harness) verifySandbox() error {
	scratch, err := os.MkdirTemp("", "leak-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	repo := filepath.Join(scratch, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return err
	}
	cmd, err := h.sandboxCmd(sandboxSpec{repo: repo, rundir: scratch, readOnly: true}, "/bin/sh", "-c", probeScript)
	if err != nil {
		return err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sandbox: %v: %s", err, out)
	}
	want := "cinch: " + sandboxCinch + "/cinch\ndocker: " + sandboxShim + "/docker"
	if got := strings.TrimSpace(string(out)); got != want {
		return fmt.Errorf("sandbox probe printed %q, want %q", got, want)
	}
	return nil
}
