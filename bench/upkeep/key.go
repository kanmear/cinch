package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// key is a task's corpus key; tasks/README.md § Key format is normative.
type key struct {
	Task    string        `json:"task"`
	Kind    string        `json:"kind"`
	Rules   []keyRule     `json:"rules"`
	Retire  []keyRetire   `json:"retire"`
	NewRule *keyCondition `json:"newRule"`
}

type keyCondition struct {
	Match  []string `json:"match"`
	Forbid []string `json:"forbid"`
}

type keyRule struct {
	ID     string   `json:"id"`
	Match  []string `json:"match"`
	Forbid []string `json:"forbid"`
}

type keyRetire struct {
	ID        string        `json:"id"`
	IfPresent *keyCondition `json:"ifPresent"`
}

// corpusRule is one rule from `cinch rules --json`.
type corpusRule struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Text    string `json:"text"`
	Markers []struct {
		File string `json:"file"`
		Line int    `json:"line"`
	} `json:"markers"`
}

func (r corpusRule) marked(underTest bool) bool {
	for _, marker := range r.Markers {
		if !underTest || strings.HasPrefix(marker.File, "test/") {
			return true
		}
	}
	return false
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

func loadKey(path string) (key, error) {
	var k key
	data, err := os.ReadFile(path)
	if err != nil {
		return k, err
	}
	return parseKey(data, path)
}

func parseKey(data []byte, name string) (key, error) {
	var k key
	if err := json.Unmarshal(data, &k); err != nil {
		return k, fmt.Errorf("%s: %w", name, err)
	}
	return k, nil
}

// satisfies reports whether text matches every pattern and none of the
// forbidden ones, case-insensitively.
func satisfies(text string, match, forbid []string) (bool, error) {
	for _, pattern := range match {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return false, fmt.Errorf("key pattern %q: %w", pattern, err)
		}
		if !re.MatchString(text) {
			return false, nil
		}
	}
	for _, pattern := range forbid {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return false, fmt.Errorf("key pattern %q: %w", pattern, err)
		}
		if re.MatchString(text) {
			return false, nil
		}
	}
	return true, nil
}

// retiredRE marks a rule that states it is retired. cinch's rule-maintenance
// workflow retires a rule by keeping its item and saying so, so a rule in a
// key's retire list whose text matches counts as retired even when ifPresent
// fails (amendment 2).
var retiredRE = regexp.MustCompile(`(?i)\bretired\b`)

// evalKey returns why the corpus fails the key; no reasons means it passes.
// pin and final are the rule inventories at the pin and on the final tree.
func evalKey(k key, pin, final map[string]corpusRule) ([]string, error) {
	var reasons []string
	for _, want := range k.Rules {
		got, ok := final[want.ID]
		if !ok {
			reasons = append(reasons, want.ID+": missing")
			continue
		}
		if !got.marked(false) {
			reasons = append(reasons, want.ID+": no marker")
			continue
		}
		ok, err := satisfies(got.Text, want.Match, want.Forbid)
		if err != nil {
			return nil, err
		}
		if !ok {
			reasons = append(reasons, want.ID+": text fails match/forbid")
		}
	}

	retired := map[string]bool{}
	for _, want := range k.Retire {
		retired[want.ID] = true
		got, present := final[want.ID]
		if !present || retiredRE.MatchString(got.Text) {
			continue
		}
		if want.IfPresent == nil {
			reasons = append(reasons, want.ID+": present and not satisfying ifPresent")
			continue
		}
		ok, err := satisfies(got.Text, want.IfPresent.Match, want.IfPresent.Forbid)
		if err != nil {
			return nil, err
		}
		if !ok {
			reasons = append(reasons, want.ID+": present and not satisfying ifPresent")
		}
	}

	if k.NewRule != nil {
		found := false
		for id, got := range final {
			if _, atPin := pin[id]; atPin || !got.marked(true) {
				continue
			}
			ok, err := satisfies(got.Text, k.NewRule.Match, nil)
			if err != nil {
				return nil, err
			}
			if ok {
				found = true
				break
			}
		}
		if !found {
			reasons = append(reasons, fmt.Sprintf("newRule: no new marked rule matching %v", k.NewRule.Match))
		}
	}

	for id := range pin {
		if retired[id] {
			continue
		}
		if got, ok := final[id]; !ok || !got.marked(false) {
			reasons = append(reasons, id+": lost (absent or unmarked)")
		}
	}
	sort.Strings(reasons)
	return reasons, nil
}

// changedRules lists rules present at the pin whose text now differs.
func changedRules(pin, final map[string]corpusRule) []string {
	var changed []string
	for id, was := range pin {
		if now, ok := final[id]; ok && now.Text != was.Text {
			changed = append(changed, id)
		}
	}
	sort.Strings(changed)
	return changed
}
