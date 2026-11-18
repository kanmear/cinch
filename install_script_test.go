package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// installStubCurl is a curl stand-in: it serves the fixture archive and
// checksums.txt from $FIXTURE_DIR and fails on anything else.
const installStubCurl = `#!/bin/sh
out=""
url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  */checksums.txt) cp "$FIXTURE_DIR/checksums.txt" "$out" ;;
  */cinch_*.tar.gz) cp "$FIXTURE_DIR/$(basename "$url")" "$out" ;;
  *) echo "stub curl: unexpected url $url" >&2; exit 22 ;;
esac
`

// cinch:rule CINCH-022
func TestInstallScriptCreatesInstallDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is not run on Windows")
	}
	for _, tool := range []string{"bash", "tar", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}

	const version = "9.9.9"
	goos, goarch := runtime.GOOS, runtime.GOARCH
	archive := "cinch_" + version + "_" + goos + "_" + goarch + ".tar.gz"

	fixture := t.TempDir()
	staging := filepath.Join(fixture, "staging")
	if err := os.Mkdir(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "cinch"), []byte("#!/bin/sh\necho cinch "+version+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run(staging, "tar", "-czf", filepath.Join(fixture, archive), "cinch")
	sums := filepath.Join(fixture, "checksums.txt")
	run(fixture, "sh", "-c", "sha256sum "+archive+" > checksums.txt")
	if _, err := os.Stat(sums); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "curl"), []byte(installStubCurl), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{"missing nested dir", func(t *testing.T) string { return filepath.Join(t.TempDir(), "nested", "bin") }},
		{"existing dir", func(t *testing.T) string { return t.TempDir() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			installDir := c.dir(t)
			cmd := exec.Command("bash", script)
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FIXTURE_DIR="+fixture,
				"CINCH_VERSION=v"+version,
				"CINCH_INSTALL_DIR="+installDir,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("install.sh: %v\n%s", err, out)
			}
			info, err := os.Stat(filepath.Join(installDir, "cinch"))
			if err != nil {
				t.Fatalf("cinch not installed: %v", err)
			}
			if info.Mode()&0o111 == 0 {
				t.Fatalf("cinch mode = %v, want executable", info.Mode())
			}
		})
	}
}
