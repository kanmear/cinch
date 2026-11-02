package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// corpusRule is one rule from `cinch rules --json`.
type corpusRule struct {
	ID      string   `json:"id"`
	File    string   `json:"file"`
	Text    string   `json:"text"`
	Markers []marker `json:"markers"`
}

type marker struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

func parseRules(data []byte) (map[string]corpusRule, error) {
	var doc struct {
		Rules []corpusRule `json:"rules"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	rules := map[string]corpusRule{}
	for _, rule := range doc.Rules {
		rules[rule.ID] = rule
	}
	return rules, nil
}

// cinchRules runs the grader's cinch in dir and returns its rule inventory.
func (h *harness) cinchRules(dir string) ([]byte, error) {
	cmd := exec.Command(h.cinchBin, "rules", "--json")
	cmd.Dir = dir
	cmd.Env = h.graderEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cinch rules --json: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !json.Valid(out) {
		return nil, fmt.Errorf("cinch rules --json: not JSON")
	}
	return out, nil
}

// pinRulesFor is the rule inventory of an arm's built state, read once.
func (h *harness) pinRulesFor(task string, arm int) (map[string]corpusRule, error) {
	key := stateKey(task, arm)
	h.pinRulesMu.Lock()
	defer h.pinRulesMu.Unlock()
	if rules, ok := h.pinRules[key]; ok {
		return rules, nil
	}
	dir, err := h.ensureState(task, arm)
	if err != nil {
		return nil, err
	}
	out, err := h.cinchRules(dir)
	if err != nil {
		return nil, err
	}
	rules, err := parseRules(out)
	if err != nil {
		return nil, err
	}
	h.pinRules[key] = rules
	return rules, nil
}

// ruleCandidate is a rule the agent wrote or changed that could count as "the
// rule" (DESIGN § Grading 3): new at the pin or changed in text, outside the
// plans directory, and marked by a test under backend/tests or frontend/tests.
type ruleCandidate struct {
	ID      string   `json:"id"`
	File    string   `json:"file"`
	Kind    string   `json:"kind"` // new or changed
	Text    string   `json:"text"`
	Markers []string `json:"markers"`            // file:line of the markers under tests
	Funcs   []string `json:"funcs"`              // file::TestFunc each marker resolves to
	AddPath bool     `json:"add_path,omitempty"` // a marked test POSTs to /members (T4)
}

func isTestFile(path string) bool {
	return strings.HasPrefix(path, "backend/tests/") || strings.HasPrefix(path, "frontend/tests/")
}

func underPlans(docsRoot, file string) bool {
	return strings.HasPrefix(file, strings.TrimSuffix(docsRoot, "/")+"/plans/")
}

// structuralCandidates lists the rules that pass the structural part of
// "rule written": new or changed, outside plans/, with a marker on a test file.
func structuralCandidates(pin, final map[string]corpusRule, docsRoot string) []ruleCandidate {
	var out []ruleCandidate
	for id, now := range final {
		kind := ""
		was, atPin := pin[id]
		switch {
		case !atPin:
			kind = "new"
		case was.Text != now.Text:
			kind = "changed"
		default:
			continue
		}
		if underPlans(docsRoot, now.File) {
			continue
		}
		c := ruleCandidate{ID: id, File: now.File, Kind: kind, Text: now.Text}
		for _, m := range now.Markers {
			if isTestFile(m.File) {
				c.Markers = append(c.Markers, fmt.Sprintf("%s:%d", m.File, m.Line))
			}
		}
		if len(c.Markers) == 0 {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

var testFuncRe = regexp.MustCompile(`^func (Test\w+)\(`)

// markedTest is a test function a rule marker sits above.
type markedTest struct {
	File, Func, Body string
}

// resolveMarkers maps each of a rule's markers under tests to the test
// function below it, skipping comment and blank lines (markers can be stacked).
func resolveMarkers(repoRoot string, r corpusRule) []markedTest {
	var out []markedTest
	byFile := map[string][]string{}
	for _, m := range r.Markers {
		if !isTestFile(m.File) {
			continue
		}
		lines, ok := byFile[m.File]
		if !ok {
			data, err := os.ReadFile(filepath.Join(repoRoot, m.File))
			if err != nil {
				continue
			}
			lines = strings.Split(string(data), "\n")
			byFile[m.File] = lines
		}
		for i := m.Line; i < len(lines); i++ { // m.Line is 1-based; lines[m.Line] is the line below
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if sub := testFuncRe.FindStringSubmatch(lines[i]); sub != nil {
				end := i + 1
				for end < len(lines) && lines[end] != "}" {
					end++
				}
				out = append(out, markedTest{File: m.File, Func: sub[1], Body: strings.Join(lines[i:min(end+1, len(lines))], "\n")})
			}
			break
		}
	}
	return out
}

var (
	invitePathRe  = regexp.MustCompile(`/members/invite`)
	membersPathRe = regexp.MustCompile(`/members(?:["'` + "`" + `?]|$|\s)`)
	addHelperRe   = regexp.MustCompile(`(?i)\b(?:post|do|send)?add_?member\w*\(`)
)

// postsToMembers reports whether a test body POSTs to the add-member path:
// `.../members` and not `.../members/invite` (T4's "bound" check). It is a
// reading of the source text, not a trace; the test names the path or calls a
// helper named for adding a member.
func postsToMembers(body string) bool {
	stripped := invitePathRe.ReplaceAllString(body, "/INVITE")
	return membersPathRe.MatchString(stripped) || addHelperRe.MatchString(stripped)
}

// newMarkedTests lists the tests a rule has in final that it did not have at
// the pin, by file and function (marker lines move when files are edited).
func newMarkedTests(pinRoot string, pin map[string]corpusRule, finalRoot string, final map[string]corpusRule, id string) []markedTest {
	was := map[string]bool{}
	if r, ok := pin[id]; ok {
		for _, t := range resolveMarkers(pinRoot, r) {
			was[t.File+"::"+t.Func] = true
		}
	}
	var out []markedTest
	if r, ok := final[id]; ok {
		for _, t := range resolveMarkers(finalRoot, r) {
			if !was[t.File+"::"+t.Func] {
				out = append(out, t)
			}
		}
	}
	return out
}

// hasTestMarker reports whether a rule has at least one marker under tests.
func hasTestMarker(r corpusRule) bool {
	for _, m := range r.Markers {
		if isTestFile(m.File) {
			return true
		}
	}
	return false
}
