// Command costcurve times cinch's pre-commit hook on synthetic repos of
// growing size, on both paths of the staged snapshot, with and without a
// large tracked binary, for one or more cinch builds. It writes one CSV row
// per cell and an env file naming the machine, since every number here is
// specific to the hardware and filesystem it ran on.
//
// Usage: go run ./bench/costcurve -cinch NAME=PATH[,NAME=PATH...] [-files 1000,10000,100000]
//
//	[-binary-mb 0,10,100] [-runs 7] [-work DIR] [-o bench/cost-curve.csv]
//
// `make cost-curve` builds the current tree and the last build before
// CINCH-009 and runs both.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Synthetic repo shape. Fixed shares, so only the file count varies.
const (
	docsShare    = 20  // one doc in every docsShare files lives under .docs
	rulesPerDoc  = 10  // rule docs carry this many numbered rules each
	ruleDocShare = 100 // one rule doc in every ruleDocShare files
	codeLines    = 30  // lines in each code file
)

type build struct{ name, path string }

func main() {
	cinches := flag.String("cinch", "", "comma-separated NAME=PATH cinch binaries to time (required)")
	filesFlag := flag.String("files", "1000,10000,100000", "comma-separated repo sizes, in tracked files")
	binaryFlag := flag.String("binary-mb", "0,10,100", "comma-separated tracked binary sizes, in MB")
	runs := flag.Int("runs", 7, "timed runs per cell")
	work := flag.String("work", defaultWork(), "directory for the synthetic repos (use the disk your real repos are on)")
	out := flag.String("o", filepath.Join("bench", "cost-curve.csv"), "CSV output; the env file goes next to it")
	flag.Parse()

	builds, err := parseBuilds(*cinches)
	if err == nil && *runs < 1 {
		err = fmt.Errorf("-runs must be at least 1")
	}
	sizes, err2 := parseInts(*filesFlag)
	binaries, err3 := parseInts(*binaryFlag)
	for _, e := range []error{err, err2, err3} {
		if e != nil {
			fail(e)
		}
	}

	if err := os.MkdirAll(*work, 0o755); err != nil {
		fail(err)
	}
	var rows []string
	for _, files := range sizes {
		repo := filepath.Join(*work, fmt.Sprintf("repo-%d", files))
		fmt.Fprintf(os.Stderr, "generating %d files in %s\n", files, repo)
		if err := generate(repo, builds[0].path, files); err != nil {
			fail(err)
		}
		for _, mb := range binaries {
			if err := setBinary(repo, mb); err != nil {
				fail(err)
			}
			for _, path := range []string{"fast", "snapshot"} {
				if err := stage(repo, path == "snapshot"); err != nil {
					fail(err)
				}
				for _, b := range builds {
					times, err := timeHook(b.path, repo, *runs)
					if err != nil {
						fail(fmt.Errorf("%s, %d files, %d MB, %s path: %w", b.name, files, mb, path, err))
					}
					row := fmt.Sprintf("%s,%d,%d,%s,%d,%d,%d,%d", b.name, files, mb, path, len(times), median(times), times[0], times[len(times)-1])
					fmt.Fprintln(os.Stderr, row)
					rows = append(rows, row)
				}
			}
		}
	}

	csv := "cinch,files,binary_mb,path,runs,median_ms,min_ms,max_ms\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(*out, []byte(csv), 0o644); err != nil {
		fail(err)
	}
	envPath := strings.TrimSuffix(*out, filepath.Ext(*out)) + ".env"
	if err := os.WriteFile(envPath, []byte(environment(*work, builds, *runs)), 0o644); err != nil {
		fail(err)
	}
}

func defaultWork() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cinch-costcurve")
	}
	return filepath.Join(os.TempDir(), "cinch-costcurve")
}

// generate builds a committed repo of exactly files tracked files: a cinch.yml,
// the workflows cinch renders, docs that link to each other, rule docs, and
// code files carrying one marker per rule. It reuses a repo whose base tag
// already holds the right count.
func generate(repo, render string, files int) error {
	if out, err := git(repo, "ls-tree", "-r", "--name-only", "base"); err == nil && strings.Count(out, "\n") == files {
		_, err := git(repo, "reset", "-q", "--hard", "base")
		return err
	}
	if err := os.RemoveAll(repo); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(repo, ".docs"), 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "bench@example.invalid"},
		{"config", "user.name", "bench"},
		{"config", "commit.gpgsign", "false"},
	} {
		if _, err := git(repo, args...); err != nil {
			return err
		}
	}

	write := func(rel, body string) error {
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(body), 0o644)
	}

	if err := write("cinch.yml", "paths:\n  docs: .docs\nrequire:\n  cinch: any\n"); err != nil {
		return err
	}
	// Rendered workflows are part of every consumer, and the generated check
	// fails without them.
	cmd := exec.Command(render, "render")
	cmd.Dir, cmd.Env = repo, cleanEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cinch render: %v\n%s", err, out)
	}

	ruleDocs := max(1, files/ruleDocShare)
	docs := max(ruleDocs+1, files/docsShare)
	var ids []string
	for d := 0; d < docs; d++ {
		var b strings.Builder
		fmt.Fprintf(&b, "# Doc %d\n\nSee [the next doc](doc-%d.md) and [the first](doc-0.md).\n\n", d, (d+1)%docs)
		if d < ruleDocs {
			for r := 1; r <= rulesPerDoc; r++ {
				id := fmt.Sprintf("R%d-%03d", d, r)
				ids = append(ids, id)
				fmt.Fprintf(&b, "%d. **%s** Rule %d of doc %d.\n", r, id, r, d)
			}
		}
		if err := write(fmt.Sprintf(".docs/doc-%d.md", d), b.String()); err != nil {
			return err
		}
	}

	existing, err := countFiles(repo)
	if err != nil {
		return err
	}
	remaining := files - existing
	if remaining < len(ids) {
		return fmt.Errorf("%d files is too few to bind %d rules", files, len(ids))
	}
	marker := "// cinch:" + "rule "
	for c := 0; c < remaining; c++ {
		var b strings.Builder
		fmt.Fprintf(&b, "package p%d\n\n", c/100)
		if c < len(ids) {
			fmt.Fprintf(&b, "%s%s\n", marker, ids[c])
		}
		for l := 0; l < codeLines; l++ {
			fmt.Fprintf(&b, "// line %d of file %d, ordinary source text\n", l, c)
		}
		if err := write(fmt.Sprintf("src/d%d/f%d.go", c/100, c), b.String()); err != nil {
			return err
		}
	}

	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-q", "--no-verify", "-m", "base"},
		{"tag", "-f", "base"},
	} {
		if _, err := git(repo, args...); err != nil {
			return err
		}
	}
	if out, err := git(repo, "ls-tree", "-r", "--name-only", "base"); err != nil || strings.Count(out, "\n") != files {
		return fmt.Errorf("generated repo holds %d tracked files, want %d", strings.Count(out, "\n"), files)
	}
	return nil
}

// countFiles counts the files under repo, outside .git.
func countFiles(repo string) (int, error) {
	n := 0
	err := filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

// setBinary commits a tracked binary of mb megabytes on top of base (none for
// 0). Its bytes are seeded pseudo-random, so it holds NULs and newlines at
// the rate a compiled binary roughly does, and every run sees the same file.
func setBinary(repo string, mb int) error {
	if _, err := git(repo, "reset", "-q", "--hard", "base"); err != nil {
		return err
	}
	if mb == 0 {
		return nil
	}
	data := make([]byte, mb<<20)
	rand.New(rand.NewSource(int64(mb))).Read(data)
	if err := os.WriteFile(filepath.Join(repo, "bin-server"), data, 0o644); err != nil {
		return err
	}
	if _, err := git(repo, "add", "bin-server"); err != nil {
		return err
	}
	_, err := git(repo, "commit", "-q", "--no-verify", "-m", "binary")
	return err
}

// stage leaves one staged doc edit, as a commit would. dirty adds an
// unstaged code edit too, which forces the hook off its fast path and onto
// a full checkout-index snapshot.
func stage(repo string, dirty bool) error {
	if _, err := git(repo, "reset", "-q", "--hard", "HEAD"); err != nil {
		return err
	}
	doc := filepath.Join(repo, ".docs", "doc-0.md")
	f, err := os.OpenFile(doc, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString("\nA staged edit.\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if _, err := git(repo, "add", ".docs/doc-0.md"); err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	code := filepath.Join(repo, "src", "d0", "f0.go")
	f, err = os.OpenFile(code, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString("// an unstaged edit\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// timeHook runs `cinch hook pre-commit` runs times after one untimed warm-up
// and returns the wall times in ms, sorted. Any non-zero exit is an error:
// the synthetic repo is valid, so a failure means the measurement is wrong.
func timeHook(cinch, repo string, runs int) ([]int, error) {
	var times []int
	for i := 0; i <= runs; i++ {
		cmd := exec.Command(cinch, "hook", "pre-commit")
		cmd.Dir = repo
		cmd.Env = cleanEnv()
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		start := time.Now()
		err := cmd.Run()
		elapsed := int(time.Since(start).Milliseconds())
		if err != nil {
			return nil, fmt.Errorf("%v\n%s", err, out.String())
		}
		if i > 0 {
			times = append(times, elapsed)
		}
	}
	sort.Ints(times)
	return times, nil
}

// cleanEnv drops GIT_* variables, so running this from inside a git hook
// can't point the timed hook at the wrong repository or index.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "NO_COLOR=1")
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func environment(work string, builds []build, runs int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "date=%s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "os=%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "cpu=%s\n", cpuModel())
	fmt.Fprintf(&b, "cpus=%d\n", runtime.NumCPU())
	fmt.Fprintf(&b, "filesystem=%s\n", filesystem(work))
	if out, err := exec.Command("git", "--version").Output(); err == nil {
		fmt.Fprintf(&b, "git=%s\n", strings.TrimSpace(strings.TrimPrefix(string(out), "git version ")))
	}
	for _, bd := range builds {
		out, _ := exec.Command(bd.path, "version").Output()
		fmt.Fprintf(&b, "cinch.%s=%s\n", bd.name, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(&b, "runs_per_cell=%d (after one untimed warm-up)\n", runs)
	return b.String()
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "model name"); ok {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), ":"))
		}
	}
	return "unknown"
}

func filesystem(dir string) string {
	out, err := exec.Command("stat", "-f", "-c", "%T", dir).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func parseBuilds(s string) ([]build, error) {
	if s == "" {
		return nil, fmt.Errorf("-cinch is required")
	}
	var builds []build
	for _, part := range strings.Split(s, ",") {
		name, path, ok := strings.Cut(part, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("-cinch entry %q is not NAME=PATH", part)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		builds = append(builds, build{name: name, path: abs})
	}
	return builds, nil
}

func parseInts(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%q is not a non-negative integer", part)
		}
		out = append(out, n)
	}
	return out, nil
}

func median(sorted []int) int {
	return sorted[len(sorted)/2]
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "costcurve:", err)
	os.Exit(1)
}
