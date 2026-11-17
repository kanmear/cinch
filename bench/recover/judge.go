package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var verdicts = []string{"CORRECT", "PARTIAL", "WRONG", "ABSTAIN"}

// rubricFields are the placeholders a rubric must contain, and the only
// things a judge is shown. Never the arm, the repo, the commit or the doc.
var rubricFields = []string{"question", "answer", "key", "notes"}

// fillRubric substitutes {{question}}, {{answer}}, {{key}} and {{notes}} into
// the rubric. A rubric without all four is refused: a judge that never sees
// the key, or the answer, grades nothing.
func fillRubric(rubric string, r result) (string, error) {
	values := map[string]string{"question": r.Question, "answer": r.Answer, "key": r.Key, "notes": r.Notes}
	for _, field := range rubricFields {
		if !strings.Contains(rubric, "{{"+field+"}}") {
			return "", fmt.Errorf("the rubric has no {{%s}} placeholder", field)
		}
	}
	// One pass, so a value that itself contains "{{key}}" is not expanded.
	pairs := make([]string, 0, 2*len(values))
	for field, value := range values {
		pairs = append(pairs, "{{"+field+"}}", value)
	}
	return strings.NewReplacer(pairs...).Replace(rubric), nil
}

var verdictLineRe = regexp.MustCompile(`^[\s*#>_` + "`" + `-]*(?:(?i:(?:final\s+)?verdict)\s*[:\-–—]?\s*)?[*_` + "`" + `]*(CORRECT|PARTIAL|WRONG|ABSTAIN)\b[*_` + "`" + `]*\s*[:.\-–—]?\s*(.*)$`)
var verdictWordRe = regexp.MustCompile(`\b(CORRECT|PARTIAL|WRONG|ABSTAIN)\b`)

// parseVerdict reads a judge's reply: exactly one of CORRECT, PARTIAL, WRONG
// or ABSTAIN (upper case), and the justification. The verdict is the first
// line that opens with one (optionally after "Verdict:" and markdown); the
// justification is the rest of that line and what follows. A reply that
// names a second, different verdict on a line of its own, or none at all, is
// an error.
func parseVerdict(reply string) (verdict, justification string, err error) {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	for i, line := range lines {
		m := verdictLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		verdict = m[1]
		rest := append([]string{strings.TrimSpace(m[2])}, lines[i+1:]...)
		justification = strings.TrimSpace(strings.Join(rest, "\n"))
		for _, later := range lines[i+1:] {
			if l := verdictLineRe.FindStringSubmatch(later); l != nil && l[1] != verdict {
				return "", "", fmt.Errorf("ambiguous verdict: %s, then %s", verdict, l[1])
			}
		}
		return verdict, justification, nil
	}
	seen := map[string]bool{}
	for _, w := range verdictWordRe.FindAllString(reply, -1) {
		seen[w] = true
	}
	if len(seen) == 1 {
		for w := range seen {
			return w, strings.TrimSpace(reply), nil
		}
	}
	return "", "", fmt.Errorf("no single verdict in the reply: %q", truncate(reply, 200))
}

var (
	// A path under the docs root: optional leading directories, .docs or
	// .agent, and what follows. The first group is the character before it, so
	// "user.agent" is not a match.
	scrubRootRe = regexp.MustCompile(`(^|[^\w.])(?:/?[\w.@~-]+/)*/?\.(?:docs|agent)\b(?:/[\w.@~-]*)*`)
	scrubMDRe   = regexp.MustCompile(`(?:/?[\w.@~-]+/)*/?[\w@~-][\w.@~-]*\.md\b`)
	scrubWordRe = regexp.MustCompile(`(?i)\b(?:documentation|documented|docs|doc)\b`)
)

// scrubAnswer blinds an answer to the arm, as DESIGN § Engine and judge says:
// any path under .docs/ or .agent/, any *.md path (AGENTS.md included) and the
// whole words doc, docs, documentation, documented become [ref]. Citations of
// code files and rule IDs stay.
func scrubAnswer(s string) string {
	s = scrubRootRe.ReplaceAllString(s, "${1}[ref]")
	s = scrubMDRe.ReplaceAllString(s, "[ref]")
	return scrubWordRe.ReplaceAllString(s, "[ref]")
}

// judger grades one row. It sees the filled rubric's inputs only.
type judger interface {
	judge(ctx context.Context, rubric string, r result) (verdict, justification string, err error)
	name() string
}

// abstainSentence is calibration set (b)'s fixed answer.
const abstainSentence = "I could not establish this from the repository."

var (
	ruleNumRe    = regexp.MustCompile(`^\s*\d+\.\s+`)
	ruleIDRe     = regexp.MustCompile(`^\*\*[A-Za-z0-9-]+\*\*\s*`)
	ruleIgnoreRe = regexp.MustCompile(`(?s)\s*<!--\s*cinch:ignore.*?-->`)
)

// ruleText is a rule as an answer would state it: the list number, the bold
// ID and any cinch:ignore comment stripped from the key.
func ruleText(key string) string {
	s := ruleIgnoreRe.ReplaceAllString(key, "")
	s = ruleNumRe.ReplaceAllString(s, "")
	s = ruleIDRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// fakeJudge is the deterministic stub the tests and dry runs use: CORRECT iff
// the answer contains the first 12 characters of the key's rule text (number,
// ID and cinch:ignore comment stripped), ABSTAIN for the calibration's fixed
// abstain sentence, else WRONG.
type fakeJudge struct{}

func (fakeJudge) name() string { return "fake" }
func (fakeJudge) judge(_ context.Context, _ string, r result) (string, string, error) {
	if strings.TrimSpace(r.Answer) == abstainSentence {
		return "ABSTAIN", "fake judge: the fixed abstain sentence", nil
	}
	prefix := ruleText(r.Key)
	if rs := []rune(prefix); len(rs) > 12 {
		prefix = string(rs[:12])
	}
	if strings.Contains(r.Answer, prefix) {
		return "CORRECT", "fake judge: the answer contains the key's first 12 characters", nil
	}
	return "WRONG", "fake judge: the answer lacks the key's first 12 characters", nil
}

// claudeJudge calls claude -p non-interactively, with no tools, in an empty
// directory with only project-level settings, so no CLAUDE.md or memory of the
// user's reaches the grader.
type claudeJudge struct {
	model   string
	bin     string
	timeout time.Duration
}

func (j claudeJudge) name() string { return j.model }

func (j claudeJudge) judge(ctx context.Context, rubric string, r result) (string, string, error) {
	prompt, err := fillRubric(rubric, r)
	if err != nil {
		return "", "", err
	}
	scratch, err := os.MkdirTemp("", "recover-judge-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(scratch)
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, j.bin, "-p", "--output-format", "json", "--model", j.model,
		"--tools", "", "--no-session-persistence", "--disable-slash-commands", "--strict-mcp-config",
		"--setting-sources", "project")
	cmd.Dir = scratch
	cmd.Env = append(cleanEnv(os.Environ()), "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1", "DISABLE_AUTOUPDATER=1")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("claude: %v: %s", err, truncate(strings.TrimSpace(stderr.String()), 300))
	}
	reply, err := replyText(stdout.Bytes())
	if err != nil {
		return "", "", err
	}
	return parseVerdict(reply)
}

// replyText extracts the model's text from claude's --output-format json: one
// result object (or an array of events whose last is the result).
func replyText(data []byte) (string, error) {
	type resultLine struct {
		Type    string `json:"type"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	var one resultLine
	if err := json.Unmarshal(data, &one); err != nil {
		var many []resultLine
		if json.Unmarshal(data, &many) != nil || len(many) == 0 {
			return "", fmt.Errorf("claude printed no JSON result: %q", truncate(string(data), 200))
		}
		one = many[len(many)-1]
	}
	if one.IsError {
		return "", fmt.Errorf("claude reported an error: %s", truncate(one.Result, 300))
	}
	if strings.TrimSpace(one.Result) == "" {
		return "", fmt.Errorf("claude returned an empty result")
	}
	return one.Result, nil
}

// judgeOne grades one answer the way every judge path does: the answer is
// scrubbed, an empty one is ABSTAIN without a call, and a failed or
// unparseable reply is retried once. It returns what the judge saw.
func judgeOne(ctx context.Context, rubric string, j judger, row result) (verdict, why, judged string, err error) {
	judged = scrubAnswer(row.Answer)
	if strings.TrimSpace(row.Answer) == "" {
		return "ABSTAIN", "no answer: the run produced none (" + row.Status + ")", judged, nil
	}
	seen := row
	seen.Answer = judged
	if verdict, why, err = j.judge(ctx, rubric, seen); err != nil {
		verdict, why, err = j.judge(ctx, rubric, seen)
	}
	return verdict, why, judged, err
}

func judgeCommand(args []string) error {
	var resultsPath, rubricPath, kind, model string
	var workers int
	var seed int64
	var pinsPath string
	fs := flag.NewFlagSet("judge", flag.ContinueOnError)
	fs.StringVar(&pinsPath, "pins", filepath.Join("bench", "recover", "PINS"), "pins: the judge order is shuffled with SCHEDULE_SEED")
	fs.Int64Var(&seed, "seed", 0, "seed for the judge order (default SCHEDULE_SEED from PINS)")
	fs.StringVar(&resultsPath, "results", "", "results CSV written by run; verdicts are added in place")
	fs.StringVar(&rubricPath, "rubric", "", "rubric file with {{question}}, {{answer}}, {{key}} and {{notes}}")
	fs.StringVar(&kind, "judge", "claude", "claude, or fake (CORRECT iff the answer contains the key's first 12 characters)")
	fs.StringVar(&model, "model", "claude-sonnet-5-5", "judge model")
	fs.IntVar(&workers, "workers", 3, "parallel judge calls")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if resultsPath == "" {
		return fmt.Errorf("-results is required")
	}
	if seed == 0 {
		if pins, err := loadPins(pinsPath); err == nil {
			seed, _ = strconv.ParseInt(pins["SCHEDULE_SEED"], 10, 64)
		}
	}
	var j judger
	switch kind {
	case "claude":
		bin, err := exec.LookPath("claude")
		if err != nil {
			return fmt.Errorf("claude: %w", err)
		}
		j = claudeJudge{model: model, bin: bin, timeout: 5 * time.Minute}
	case "fake":
		j = fakeJudge{}
	default:
		return fmt.Errorf("-judge wants claude or fake, got %q", kind)
	}
	rubric := ""
	if kind == "claude" && rubricPath == "" {
		return fmt.Errorf("-rubric is required with -judge claude")
	}
	if rubricPath != "" {
		data, err := os.ReadFile(rubricPath)
		if err != nil {
			return fmt.Errorf("rubric: %w", err)
		}
		rubric = string(data)
	}
	ctx := context.Background()
	return judgeAll(ctx, resultsPath, rubric, j, workers, seed)
}

// judgeAll grades every row without a verdict, saving the file after each call
// so an interrupted or failed judge resumes where it stopped. A row whose
// reply cannot be parsed keeps no verdict and a judge_error, and is retried by
// the next invocation.
func judgeAll(ctx context.Context, path, rubric string, j judger, workers int, seed int64) error {
	rows, err := readCSV(path)
	if err != nil {
		return err
	}
	var todo []int
	for i, r := range rows {
		if r.Verdict == "" {
			todo = append(todo, i)
		}
	}
	// Every answer of the file, both arms, in a seeded order, so no arm is
	// judged in a block.
	shuffle(seed, "judge", len(todo), func(a, b int) { todo[a], todo[b] = todo[b], todo[a] })
	logf("%d of %d answers to judge", len(todo), len(rows))
	var mu sync.Mutex
	save := func() error {
		data, err := encodeCSV(rows)
		if err != nil {
			return err
		}
		return writeFileAtomic(path, data)
	}
	if workers < 1 {
		workers = 1
	}
	queue := make(chan int)
	var wg sync.WaitGroup
	var saveErr error
	failed := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				mu.Lock()
				row := rows[i]
				mu.Unlock()
				// The judge sees the scrubbed answer; the CSV keeps both.
				verdict, why, judged, jerr := judgeOne(ctx, rubric, j, row)
				row.JudgedAnswer = judged
				mu.Lock()
				rows[i].JudgedAnswer = row.JudgedAnswer
				if jerr != nil {
					rows[i].JudgeError = jerr.Error()
					failed++
					logf("%s: judge failed: %v", row.RunID, jerr)
				} else {
					rows[i].Verdict, rows[i].Justification, rows[i].JudgeError, rows[i].JudgeModel = verdict, why, "", j.name()
				}
				if err := save(); err != nil && saveErr == nil {
					saveErr = err
				}
				mu.Unlock()
			}
		}()
	}
	for _, i := range todo {
		if ctx.Err() != nil {
			break
		}
		queue <- i
	}
	close(queue)
	wg.Wait()
	if saveErr != nil {
		return saveErr
	}
	if len(todo) == 0 {
		return nil
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d judge calls failed; run judge again to retry them", failed, len(todo))
	}
	return ctx.Err()
}
