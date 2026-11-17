package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// canonicalTasks are the four tasks in schedule order. T4 is withheld: its
// files live under private/ (git-ignored) and its results go to a private CSV.
var canonicalTasks = []string{"T1", "T2", "T3", "T4"}

// defaultArms are the arms a task has unless PINS says otherwise with
// <TASK>_ARMS (a comma-separated list), e.g. T4_ARMS=1,3.
var defaultArms = []int{1, 2, 3}

// loadPins reads the KEY=VALUE pin file.
func loadPins(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pins: %w", err)
	}
	defer file.Close()
	pins := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("pins: %s: not KEY=VALUE: %q", path, line)
		}
		pins[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return pins, scanner.Err()
}

// pinTODO is true for a value that is missing or still a placeholder.
func pinTODO(pins map[string]string, key string) bool {
	v := pins[key]
	return v == "" || strings.EqualFold(v, "TODO")
}

// requirePins refuses to go on while any of the keys is missing or TODO.
func requirePins(pins map[string]string, keys ...string) error {
	var missing []string
	for _, key := range keys {
		if pinTODO(pins, key) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("PINS has no value for %s (TODO): the owner fills these at prereg/leak-pilot or prereg/leak before a run", strings.Join(missing, ", "))
	}
	return nil
}

func pinInt(pins map[string]string, key string, fallback int) int {
	if n, err := strconv.Atoi(pins[key]); err == nil {
		return n
	}
	return fallback
}

// armsOf is the arms a task runs, from <TASK>_ARMS or the default.
func armsOf(pins map[string]string, task string) []int {
	raw := pins[task+"_ARMS"]
	if raw == "" {
		return defaultArms
	}
	var out []int
	for _, part := range splitList(raw) {
		if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= 3 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return defaultArms
	}
	return out
}

// stateArm is the arm whose constructed state a run uses: arm 3 shares arm 1's.
func stateArm(arm int) int {
	if arm == 3 {
		return 1
	}
	return arm
}

// treePinKey is the PINS key holding a state's committed `git ls-tree -r HEAD`
// sha256.
func treePinKey(task string, arm int) string {
	return fmt.Sprintf("%s_ARM%d_TREE_SHA256", task, stateArm(arm))
}

// taskDir finds a task's directory under tasks/ (public) or private/tasks/
// (withheld): the one named "<ID>" or "<ID>-*".
func taskDir(leakDir, id string) (dir string, private bool, err error) {
	for _, base := range []struct {
		path    string
		private bool
	}{{filepath.Join(leakDir, "tasks"), false}, {filepath.Join(leakDir, "private", "tasks"), true}} {
		matches, _ := filepath.Glob(filepath.Join(base.path, id+"-*"))
		matches = append([]string{filepath.Join(base.path, id)}, matches...) // the exact <ID> directory too
		for _, m := range matches {
			if st, err := os.Stat(m); err == nil && st.IsDir() {
				return m, base.private, nil
			}
		}
	}
	return "", false, fmt.Errorf("no directory for task %s under %s/tasks or %s/private/tasks", id, leakDir, leakDir)
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func contains(list []string, item string) bool {
	for _, existing := range list {
		if existing == item {
			return true
		}
	}
	return false
}

func containsInt(list []int, item int) bool {
	for _, existing := range list {
		if existing == item {
			return true
		}
	}
	return false
}

func appendUnique(list []string, item string) []string {
	if contains(list, item) {
		return list
	}
	return append(list, item)
}
