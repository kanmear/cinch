package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// shimRefusal is what every call the shim does not answer prints.
const shimRefusal = "docker is not available in this workspace"

// shimExists is what the backend Makefile's `test` target gets back for its one
// docker call, `docker-compose exec -T postgres createdb -U <user> deltadocs_test`.
// The Makefile continues on "already exists", so `make test-backend` reaches
// `go test` against TEST_DATABASE_URL, the run's own work database.
const shimExists = `createdb: error: database "deltadocs_test" already exists`

// composeShim is the docker-compose stand-in (DESIGN § Databases). It answers
// exactly one invocation and refuses the rest, so `make dev-db` (which pauses
// whatever container publishes 5432), `docker ps`, `docker-compose up` and the
// rest all fail.
func composeShim(pgUser string) string {
	return `#!/bin/sh
# docker-compose stand-in for the benchmark sandbox (bench/leak/shim.go).
case "$*" in
  "exec -T postgres createdb -U ` + pgUser + ` deltadocs_test")
    echo '` + shimExists + `' >&2
    exit 1
    ;;
esac
echo '` + shimRefusal + `' >&2
exit 1
`
}

// dockerShim refuses everything.
func dockerShim() string {
	return `#!/bin/sh
# docker stand-in for the benchmark sandbox (bench/leak/shim.go).
echo '` + shimRefusal + `' >&2
exit 1
`
}

// shimNames are the executables the sandbox answers with a shim, wherever they
// are looked up: first on PATH, and bound over the real binaries in /usr/bin.
var shimNames = []string{"docker", "docker-compose"}

// writeShims writes the shim scripts into dir/bin and returns that directory.
func writeShims(dir, pgUser string) (string, error) {
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	for name, body := range map[string]string{"docker-compose": composeShim(pgUser), "docker": dockerShim()} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			return "", fmt.Errorf("shim %s: %w", name, err)
		}
	}
	return bin, nil
}
