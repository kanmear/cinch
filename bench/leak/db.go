package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// pgConfig is the benchmark Postgres: the cinch-binding-pg container on host
// port 55432 (bench/binding). Port 5432 is held by other work and is never
// touched: ensureRunning refuses a container that publishes it.
type pgConfig struct {
	container, user string
	port            int
	// docker is the docker binary of the host (the sandbox has none).
	docker string
}

func newPG(pins map[string]string) pgConfig {
	port := pinInt(pins, "PG_PORT", 55432)
	container := pins["PG_CONTAINER"]
	if container == "" {
		container = "cinch-binding-pg"
	}
	user := pins["PG_USER"]
	if user == "" {
		user = "deltadocs_user"
	}
	return pgConfig{container: container, user: user, port: port, docker: "docker"}
}

func (p pgConfig) run(args ...string) ([]byte, error) {
	cmd := exec.Command(p.docker, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.Bytes(), err
}

// ensureRunning checks the container exists, starts it when it is stopped
// (it is the benchmark's own, on its own port), and refuses one that
// publishes 5432. It never touches any other container.
func (p pgConfig) ensureRunning() error {
	if p.port == 5432 {
		return fmt.Errorf("PG_PORT is 5432, which other work holds: refusing")
	}
	out, err := p.run("inspect", p.container, "--format", "{{.State.Running}} {{json .HostConfig.PortBindings}}")
	if err != nil {
		return fmt.Errorf("is the %s container created? %v: %s", p.container, err, strings.TrimSpace(string(out)))
	}
	line := strings.TrimSpace(string(out))
	if strings.Contains(line, `"HostPort":"5432"`) {
		return fmt.Errorf("container %s publishes host port 5432: refusing", p.container)
	}
	if !strings.Contains(line, fmt.Sprintf(`"HostPort":"%d"`, p.port)) {
		return fmt.Errorf("container %s does not publish host port %d: %s", p.container, p.port, line)
	}
	if !strings.HasPrefix(line, "true") {
		logf("starting the benchmark Postgres container %s", p.container)
		if out, err := p.run("start", p.container); err != nil {
			return fmt.Errorf("docker start %s: %v: %s", p.container, err, strings.TrimSpace(string(out)))
		}
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := p.run("exec", p.container, "pg_isready", "-U", p.user); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s never became ready", p.container)
}

// dbNameRe keeps database names to what createdb takes without quoting.
var dbNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// createDB makes a fresh, empty database, dropping a leftover of the same name.
func (p pgConfig) createDB(name string) error {
	if !dbNameRe.MatchString(name) {
		return fmt.Errorf("bad database name %q", name)
	}
	if err := p.dropDB(name); err != nil {
		return err
	}
	if out, err := p.run("exec", p.container, "createdb", "-U", p.user, name); err != nil {
		return fmt.Errorf("createdb %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p pgConfig) dropDB(name string) error {
	if !dbNameRe.MatchString(name) {
		return fmt.Errorf("bad database name %q", name)
	}
	if out, err := p.run("exec", p.container, "dropdb", "-U", p.user, "--if-exists", "--force", name); err != nil {
		return fmt.Errorf("dropdb %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

var urlLineRe = regexp.MustCompile(`(?m)^TEST_DATABASE_URL=(\S+)`)

// testURLTemplate reads TEST_DATABASE_URL out of the state's backend/.env.example.
func testURLTemplate(repo string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repo, "backend", ".env.example"))
	if err != nil {
		return "", err
	}
	m := urlLineRe.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("no TEST_DATABASE_URL in %s/backend/.env.example", repo)
	}
	return string(m[1]), nil
}

// databaseURL swaps the template's host, port and database for the benchmark's:
// 127.0.0.1, the pinned port and db. User and password stay the template's.
func databaseURL(template, db string, port int) (string, error) {
	u, err := url.Parse(template)
	if err != nil {
		return "", err
	}
	u.Host = fmt.Sprintf("127.0.0.1:%d", port)
	u.Path = "/" + db
	return u.String(), nil
}

// runDBName names a run's database: leak_<engine>_<task>_a<arm>_r<rep>_<role>,
// with a pilot and an attempt marker so reruns never share one.
func runDBName(engine string, j job, attempt int, role string) string {
	e := strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(engine))
	mid := ""
	if j.Pilot {
		mid = "p"
	}
	if attempt > 1 {
		mid += fmt.Sprintf("x%d", attempt)
	}
	return fmt.Sprintf("leak_%s%s_%s_a%d_r%d_%s", e, mid, strings.ToLower(j.Task), j.Arm, j.Index, role)
}
