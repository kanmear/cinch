package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// result is what one run records in result.json and the CSV; the judge adds
// judged_answer, verdict and the rest to the CSV.
type result struct {
	RunID         string  `json:"run_id"`
	Repo          string  `json:"repo"`
	Commit        string  `json:"commit"`
	Engine        string  `json:"engine"`
	QuestionID    string  `json:"question_id"`
	Doc           string  `json:"doc"`
	Ignored       bool    `json:"ignored,omitempty"`
	Arm           string  `json:"arm"`
	Rep           int     `json:"rep"`
	Position      int     `json:"position"`
	Seed          int64   `json:"seed"`
	Predict       string  `json:"predict"`
	Status        string  `json:"status"`
	TimedOut      bool    `json:"timed_out"`
	ExitCode      int     `json:"exit_code"`
	WallMS        int64   `json:"wall_ms"`
	EngineError   string  `json:"engine_error,omitempty"`
	Turns         int     `json:"turns"`
	Input         int     `json:"input_tokens"`
	Output        int     `json:"output_tokens"`
	CacheRead     int     `json:"cache_read"`
	CacheWrite    int     `json:"cache_write"`
	ToolCalls     int     `json:"tool_calls"`
	CostUSD       float64 `json:"cost_usd"`
	Question      string  `json:"question"`
	Key           string  `json:"key"`
	Notes         string  `json:"notes,omitempty"`
	Answer        string  `json:"answer"`
	JudgedAnswer  string  `json:"judged_answer,omitempty"`
	Verdict       string  `json:"verdict,omitempty"`
	Justification string  `json:"justification,omitempty"`
	JudgeError    string  `json:"judge_error,omitempty"`
	JudgeModel    string  `json:"judge_model,omitempty"`
}

// column maps one CSV column to its field, so reading and writing cannot drift.
type column struct {
	name string
	get  func(*result) string
	set  func(*result, string)
}

func str(name string, field func(*result) *string) column {
	return column{name, func(r *result) string { return *field(r) }, func(r *result, v string) { *field(r) = v }}
}

func num(name string, field func(*result) *int) column {
	return column{name, func(r *result) string { return strconv.Itoa(*field(r)) },
		func(r *result, v string) { *field(r), _ = strconv.Atoi(v) }}
}

var columns = []column{
	str("run_id", func(r *result) *string { return &r.RunID }),
	str("repo", func(r *result) *string { return &r.Repo }),
	str("commit", func(r *result) *string { return &r.Commit }),
	str("engine", func(r *result) *string { return &r.Engine }),
	str("question_id", func(r *result) *string { return &r.QuestionID }),
	str("doc", func(r *result) *string { return &r.Doc }),
	{"ignored", func(r *result) string { return strconv.FormatBool(r.Ignored) },
		func(r *result, v string) { r.Ignored = v == "true" }},
	str("arm", func(r *result) *string { return &r.Arm }),
	num("rep", func(r *result) *int { return &r.Rep }),
	num("position", func(r *result) *int { return &r.Position }),
	{"seed", func(r *result) string { return strconv.FormatInt(r.Seed, 10) },
		func(r *result, v string) { r.Seed, _ = strconv.ParseInt(v, 10, 64) }},
	str("predict", func(r *result) *string { return &r.Predict }),
	str("status", func(r *result) *string { return &r.Status }),
	{"timed_out", func(r *result) string { return strconv.FormatBool(r.TimedOut) },
		func(r *result, v string) { r.TimedOut = v == "true" }},
	num("exit_code", func(r *result) *int { return &r.ExitCode }),
	{"wall_s", func(r *result) string { return strconv.FormatInt(r.WallMS/1000, 10) },
		func(r *result, v string) { s, _ := strconv.ParseInt(v, 10, 64); r.WallMS = s * 1000 }},
	str("engine_error", func(r *result) *string { return &r.EngineError }),
	num("turns", func(r *result) *int { return &r.Turns }),
	num("input_tokens", func(r *result) *int { return &r.Input }),
	num("output_tokens", func(r *result) *int { return &r.Output }),
	num("cache_read", func(r *result) *int { return &r.CacheRead }),
	num("cache_write", func(r *result) *int { return &r.CacheWrite }),
	num("tool_calls", func(r *result) *int { return &r.ToolCalls }),
	{"cost_usd", func(r *result) string { return fmt.Sprintf("%.4f", r.CostUSD) },
		func(r *result, v string) { r.CostUSD, _ = strconv.ParseFloat(v, 64) }},
	str("question", func(r *result) *string { return &r.Question }),
	str("key", func(r *result) *string { return &r.Key }),
	str("notes", func(r *result) *string { return &r.Notes }),
	str("answer", func(r *result) *string { return &r.Answer }),
	str("judged_answer", func(r *result) *string { return &r.JudgedAnswer }),
	str("verdict", func(r *result) *string { return &r.Verdict }),
	str("justification", func(r *result) *string { return &r.Justification }),
	str("judge_error", func(r *result) *string { return &r.JudgeError }),
	str("judge_model", func(r *result) *string { return &r.JudgeModel }),
}

// encodeCSV renders rows with the header.
func encodeCSV(rows []result) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := make([]string, len(columns))
	for i, c := range columns {
		header[i] = c.name
	}
	writer.Write(header)
	for i := range rows {
		record := make([]string, len(columns))
		for k, c := range columns {
			record[k] = c.get(&rows[i])
		}
		writer.Write(record)
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

// readCSV reads rows by column name, so an older file with fewer columns, or
// a different order, still loads.
func readCSV(path string) ([]result, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s: empty", path)
	}
	index := map[string]int{}
	for i, name := range records[0] {
		index[name] = i
	}
	for _, need := range []string{"run_id", "question_id", "arm"} {
		if _, ok := index[need]; !ok {
			return nil, fmt.Errorf("%s: no %s column", path, need)
		}
	}
	var rows []result
	for _, record := range records[1:] {
		var row result
		for _, c := range columns {
			if i, ok := index[c.name]; ok && i < len(record) {
				c.set(&row, record[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// writeFileAtomic replaces path whole, so an interrupted write never leaves a
// half-written results file.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readResult(dir string) (result, error) {
	var res result
	data, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		return res, err
	}
	return res, json.Unmarshal(data, &res)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// writeCSV rebuilds the table from every result.json under the runs dir,
// keeping the verdicts a judge already wrote into the file at -o.
func (h *harness) writeCSV() error {
	h.csvMutex.Lock()
	defer h.csvMutex.Unlock()
	paths, err := filepath.Glob(filepath.Join(h.runsDir, "*", "result.json"))
	if err != nil {
		return err
	}
	var all []result
	for _, path := range paths {
		res, err := readResult(filepath.Dir(path))
		if err != nil {
			return err
		}
		all = append(all, res)
	}
	if prior, err := readCSV(h.out); err == nil {
		verdicts := map[string]result{}
		for _, p := range prior {
			verdicts[p.RunID] = p
		}
		for i := range all {
			if p, ok := verdicts[all[i].RunID]; ok && p.Answer == all[i].Answer {
				all[i].Verdict, all[i].Justification, all[i].JudgeError, all[i].JudgeModel, all[i].JudgedAnswer = p.Verdict, p.Justification, p.JudgeError, p.JudgeModel, p.JudgedAnswer
			}
		}
	}
	sortResults(all)
	data, err := encodeCSV(all)
	if err != nil {
		return err
	}
	return writeFileAtomic(h.out, data)
}
