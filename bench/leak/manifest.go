package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The private manifest (bench/private-manifest.txt, shared by the benchmarks
// that keep files out of the public history) holds a sha256 of every private
// file in sha256sum format, paths relative to bench/. `leak manifest` rewrites
// the lines under leak/private/ and keeps the rest; the harness refuses to run
// a private task whose files differ from it.

// privateFiles lists the pinned files under <leakDir>/private, as paths relative
// to bench/ (so "leak/private/tasks/..."), mapped to their sha256. The
// harness's own outputs (result CSVs) and the .gitignore are not pinned.
func privateFiles(leakDir string) (map[string]string, error) {
	root := filepath.Join(leakDir, "private")
	prefix := leakBase(leakDir) + "/private/"
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		base := filepath.Base(path)
		if base == ".gitignore" || (strings.HasSuffix(base, ".csv") && strings.Contains(base, "results-")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[prefix+filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}

// leakBase is the directory's own name ("leak"), whatever way it was spelled.
func leakBase(leakDir string) string {
	abs, err := filepath.Abs(leakDir)
	if err != nil {
		abs = filepath.Clean(leakDir)
	}
	return filepath.Base(abs)
}

func manifestPath(leakDir string) string {
	abs, err := filepath.Abs(leakDir)
	if err != nil {
		abs = filepath.Clean(leakDir)
	}
	return filepath.Join(filepath.Dir(abs), "private-manifest.txt")
}

// readManifest returns the header comments and the path -> sha256 lines.
func readManifest(path string) (comments []string, lines map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	lines = map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "#"):
			comments = append(comments, line)
		default:
			sum, name, ok := strings.Cut(line, "  ")
			if !ok {
				return nil, nil, fmt.Errorf("%s: not sha256sum format: %q", path, line)
			}
			lines[strings.TrimSpace(name)] = strings.TrimSpace(sum)
		}
	}
	return comments, lines, sc.Err()
}

// verifyPrivate refuses to go on when a private task is scheduled and a file
// under private/ differs from the manifest (or is missing from it). Tasks
// outside private/ need nothing.
func (h *harness) verifyPrivate(tasks []string) error {
	needed := false
	for _, task := range tasks {
		if _, private, err := taskDir(h.leakDir, task); err == nil && private {
			needed = true
		}
	}
	if !needed || h.allowUnpinned {
		return nil
	}
	_, pinned, err := readManifest(manifestPath(h.leakDir))
	if err != nil {
		return fmt.Errorf("private manifest: %w (run `leak manifest`, or -allow-unpinned while calibrating)", err)
	}
	files, err := privateFiles(h.leakDir)
	if err != nil {
		return err
	}
	var problems []string
	for name, sum := range files {
		switch want, ok := pinned[name]; {
		case !ok:
			problems = append(problems, name+": not in the manifest")
		case want != sum:
			problems = append(problems, name+": differs from the manifest")
		}
	}
	prefix := leakBase(h.leakDir) + "/private/"
	for name := range pinned {
		if _, ok := files[name]; !ok && strings.HasPrefix(name, prefix) {
			problems = append(problems, name+": in the manifest but missing")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("private files do not match bench/private-manifest.txt:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func manifestMain(args []string) error {
	var leakDir string
	var check bool
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	fs.StringVar(&leakDir, "leak-dir", filepath.Join("bench", "leak"), "bench/leak")
	fs.BoolVar(&check, "check", false, "verify instead of write: exit non-zero on any difference")
	if err := fs.Parse(args); err != nil {
		return err
	}
	h := &harness{config: config{leakDir: leakDir}}
	if check {
		return h.verifyPrivate(canonicalTasks)
	}
	path := manifestPath(leakDir)
	comments, existing, err := readManifest(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if existing == nil {
		existing = map[string]string{}
	}
	prefix := leakBase(leakDir) + "/private/"
	for name := range existing {
		if strings.HasPrefix(name, prefix) {
			delete(existing, name)
		}
	}
	files, err := privateFiles(leakDir)
	if err != nil {
		return err
	}
	for name, sum := range files {
		existing[name] = sum
	}
	if len(comments) == 0 {
		comments = []string{"# sha256 of private benchmark files kept out of the public history (plan 29)."}
	}
	names := make([]string, 0, len(existing))
	for name := range existing {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, c := range comments {
		b.WriteString(c + "\n")
	}
	for _, name := range names {
		b.WriteString(existing[name] + "  " + name + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s: %d private leak files\n", path, len(files))
	return nil
}
