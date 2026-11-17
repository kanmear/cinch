package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// question is one entry of a questions file: the rule it probes, the question
// an agent is asked, and the key a judge grades against. Predict is the
// author's prediction, kept to compare with what the census observes.
type question struct {
	ID       string `json:"id"`
	Doc      string `json:"doc"`
	Question string `json:"question"`
	Key      string `json:"key"`
	Notes    string `json:"notes"`
	Predict  string `json:"predict"`
	Ignored  bool   `json:"ignored"`
}

// questionFile is questions/<repo>.json.
type questionFile struct {
	Repo      string     `json:"repo"`
	Commit    string     `json:"commit"`
	Questions []question `json:"questions"`
}

func loadQuestions(path string) (*questionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseQuestions(data)
}

// parseQuestions is strict: unknown fields, a missing key, a repeated ID or an
// unknown prediction is an error, because the file is pre-registered.
func parseQuestions(data []byte) (*questionFile, error) {
	var f questionFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("questions: %w", err)
	}
	if f.Repo == "" || f.Commit == "" {
		return nil, fmt.Errorf("questions: repo and commit are required")
	}
	if len(f.Questions) == 0 {
		return nil, fmt.Errorf("questions: no questions")
	}
	seen := map[string]bool{}
	for i, q := range f.Questions {
		where := fmt.Sprintf("questions[%d] (%q)", i, q.ID)
		if q.ID == "" {
			return nil, fmt.Errorf("questions[%d]: id is required", i)
		}
		if seen[q.ID] {
			return nil, fmt.Errorf("%s: duplicate id", where)
		}
		seen[q.ID] = true
		if q.Ignored {
			continue
		}
		if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.Key) == "" || q.Doc == "" {
			return nil, fmt.Errorf("%s: doc, question and key are required", where)
		}
		if q.Predict != "recoverable" && q.Predict != "normative" {
			return nil, fmt.Errorf("%s: predict is %q, want recoverable or normative", where, q.Predict)
		}
	}
	return &f, nil
}

// selected are the questions that run, in file order: the active ones, plus
// the ignored ones when asked for (DESIGN reports them as a secondary row).
// An ignored question that runs must be complete.
func (f *questionFile) selected(includeIgnored bool) ([]question, error) {
	var out []question
	for _, q := range f.Questions {
		if q.Ignored {
			if !includeIgnored {
				continue
			}
			if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.Key) == "" || q.Doc == "" ||
				(q.Predict != "recoverable" && q.Predict != "normative") {
				return nil, fmt.Errorf("ignored question %s is incomplete (doc, question, key and predict are required to run it)", q.ID)
			}
		}
		out = append(out, q)
	}
	return out, nil
}

// active are the questions that run by default, in file order.
func (f *questionFile) active() []question {
	var out []question
	for _, q := range f.Questions {
		if !q.Ignored {
			out = append(out, q)
		}
	}
	return out
}

// pinPrefix is the PINS key prefix of a repo: budgetron -> BUDGETRON.
func pinPrefix(repo string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(repo) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
