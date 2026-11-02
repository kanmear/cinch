package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// regradeAll re-evaluates the corpus key of every graded run from the
// rules.json the grader saved; nothing reruns, and behavior stands as graded.
// With the current keys it updates result.json, and a run whose corpus or
// upkeep changes keeps its first grade in corpus_first and upkeep_first. With
// -keys-ref the keys come from that git ref and only the CSV is written, so a
// key amendment can be reported next to the result it would have given.
func (h *harness) regradeAll() error {
	h.runsDir = filepath.Join(h.work, "runs", h.runName())
	pinFile := filepath.Join(h.work, "pin-rules-"+h.pins["FIXTURE_SHA"][:12]+".json")
	data, err := os.ReadFile(pinFile)
	if err != nil {
		return fmt.Errorf("pin rules (run the harness once to build them): %w", err)
	}
	pin, err := parseRules(data)
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(h.runsDir, "*", "result.json"))
	if err != nil {
		return err
	}
	keys := map[string]key{}
	var all []result
	changed := 0
	for _, path := range paths {
		dir := filepath.Dir(path)
		res, err := readResult(dir)
		if err != nil {
			return err
		}
		rules, err := os.ReadFile(filepath.Join(dir, "rules.json"))
		if res.Arm > 1 && err == nil {
			k, ok := keys[res.Task]
			if !ok {
				if k, err = h.taskKey(res.Task); err != nil {
					return err
				}
				keys[res.Task] = k
			}
			final, err := parseRules(rules)
			if err != nil {
				return fmt.Errorf("%s: %w", dir, err)
			}
			reasons, err := evalKey(k, pin, final)
			if err != nil {
				return err
			}
			corpus := passFail(len(reasons) == 0)
			upkeep := passFail(res.Behavior == "pass" && corpus == "pass")
			if corpus != res.Corpus || upkeep != res.Upkeep {
				logf("%s: corpus %s -> %s, upkeep %s -> %s", res.RunID, res.Corpus, corpus, res.Upkeep, upkeep)
				if h.keysRef == "" && res.CorpusFirst == "" {
					res.CorpusFirst, res.UpkeepFirst = res.Corpus, res.Upkeep
				}
				changed++
			}
			res.Corpus, res.CorpusReasons, res.Upkeep = corpus, reasons, upkeep
			if h.keysRef == "" {
				if err := writeJSON(path, res); err != nil {
					return err
				}
			}
		}
		all = append(all, res)
	}
	logf("regraded %d runs in %s, %d changed; table %s", len(all), h.runsDir, changed, h.out)
	return h.writeTable(all)
}

// taskKey reads a task's key from the tasks dir, or from -keys-ref.
func (h *harness) taskKey(task string) (key, error) {
	if h.keysRef == "" {
		return loadKey(filepath.Join(h.tasksDir, task, "key.json"))
	}
	spec := h.keysRef + ":./" + task + "/key.json"
	cmd := exec.Command("git", "show", spec)
	cmd.Dir = h.tasksDir
	out, err := cmd.Output()
	if err != nil {
		return key{}, fmt.Errorf("git show %s: %v", spec, strings.TrimSpace(fmt.Sprint(err)))
	}
	return parseKey(out, spec)
}
