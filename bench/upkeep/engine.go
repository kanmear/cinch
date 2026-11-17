package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// toolCall is one tool use the agent made, whichever engine reported it.
type toolCall struct {
	ID, Name string
	Command  string // a shell tool's command line
	Path     string // a file tool's target
	Input    string // the call's arguments, as JSON
	Result   string // the tool's output, truncated
}

// trace is what a transcript says happened.
type trace struct {
	Calls                           []toolCall
	Turns, Input, Output, CacheRead int
	CacheWrite                      int
	CostUSD                         float64
	FinalText                       string
	EngineError                     string
	sawResult                       bool
}

// agentRun is how the agent process ended.
type agentRun struct {
	ExitCode int
	TimedOut bool
	WallMS   int64
	Ran      bool // false for engines that run no process
}

// runAgent starts the engine in a sandbox over repo with the task prompt,
// verbatim, and writes its raw stdout to transcript.jsonl and its stderr to
// stderr.txt. A timeout kills the sandbox's whole process group; the run is
// graded as it stands, never retried. The fake engine runs a script in the
// same sandbox in place of a model.
func (h *harness) runAgent(ctx context.Context, repo, rundir string, arm int, prompt string) (agentRun, error) {
	timeout, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	spec := sandboxSpec{arm: arm, repo: repo, rundir: rundir, engine: h.engine}
	var argv []string
	var stdin io.Reader
	switch h.engine {
	case "pi":
		argv = []string{h.sandbox.piNode, h.sandbox.piCLI, "-p", "--mode", "json",
			"--provider", h.provider, "--model", h.model, "--thinking", h.thinking,
			"--no-session", "--offline", "--approve", "-ne", "-ns", "-np",
			"-t", "read,bash,edit,write,grep,find,ls", "--", prompt}
	case "claude":
		argv = []string{h.sandbox.claudeBin, "-p", "--output-format", "stream-json", "--verbose",
			"--model", h.claudeModel, "--max-turns", fmt.Sprint(h.maxTurns),
			"--dangerously-skip-permissions", "--no-session-persistence",
			"--setting-sources", "project", "--strict-mcp-config", "--disable-slash-commands"}
		stdin = strings.NewReader(prompt)
	case "fake":
		argv = []string{"/bin/bash", sandboxScript}
	default:
		return agentRun{}, fmt.Errorf("engine %q runs no process", h.engine)
	}
	built, err := h.sandboxCmd(spec, argv...)
	if err != nil {
		return agentRun{}, err
	}
	cmd := exec.CommandContext(timeout, built.Path, built.Args[1:]...)
	cmd.Env, cmd.Stdin = built.Env, stdin
	// The agent spawns its own children (flutter, dart); kill the whole group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second

	transcript, err := os.Create(filepath.Join(rundir, "transcript.jsonl"))
	if err != nil {
		return agentRun{}, err
	}
	defer transcript.Close()
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = transcript, &stderr

	start := time.Now()
	err = cmd.Run()
	ended := agentRun{WallMS: time.Since(start).Milliseconds(), Ran: true}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // stray children of a finished agent
	}
	if ctx.Err() != nil {
		return ended, ctx.Err() // the harness was interrupted, not the run
	}
	ended.TimedOut = timeout.Err() != nil
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		ended.ExitCode = exit.ExitCode()
	case err != nil && !ended.TimedOut:
		return ended, fmt.Errorf("%s: %w", h.engine, err)
	}
	if stderr.Len() > 0 {
		if err := os.WriteFile(filepath.Join(rundir, "stderr.txt"), stderr.Bytes(), 0o644); err != nil {
			return ended, err
		}
	}
	return ended, nil
}

// applyPatch is the "agent" of -apply: it applies a reference patch, as a
// real agent would regenerate sources and commit, in the arm's sandbox and
// with the arm's hooks active. Arm 1 has no docs, so the patch's .docs hunks
// are left out, and -apply-strip-docs leaves them out in every arm.
func (h *harness) applyPatch(repo, rundir, patch string, arm int) error {
	var log bytes.Buffer
	step := func(argv ...string) error {
		cmd, err := h.sandboxCmd(sandboxSpec{arm: arm, repo: repo, rundir: rundir}, argv...)
		if err != nil {
			return err
		}
		out, err := cmd.CombinedOutput()
		fmt.Fprintf(&log, "$ %s\n%s\n", strings.Join(argv, " "), out)
		return err
	}
	defer func() { _ = os.WriteFile(filepath.Join(rundir, "apply.log"), log.Bytes(), 0o644) }()
	// The patch sits outside the checkout; the sandbox sees only the checkout.
	data, err := os.ReadFile(patch)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "bench-reference.patch"), data, 0o644); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(repo, ".git", "bench-reference.patch"))
	args := []string{"git", "apply", "--whitespace=nowarn"}
	if arm == 1 || h.engine == "apply-strip-docs" {
		args = append(args, "--exclude=.docs/*")
	}
	if err := step(append(args, sandboxWork+"/.git/bench-reference.patch")...); err != nil {
		return fmt.Errorf("git apply %s: %w", patch, err)
	}
	if err := step("make", "gen"); err != nil {
		return fmt.Errorf("make gen after the patch: %w", err)
	}
	if err := step("git", "add", "-A"); err != nil {
		return err
	}
	// A blocked commit leaves the tree uncommitted; that is a result, not an error.
	_ = step("git", "commit", "-q", "-m", "feat: reference solution")
	return nil
}

// parseTranscript reads an engine's raw output into tool calls, usage and the
// final text. Usage is summed over assistant message_end events for pi (the
// other events repeat them) and read from the result line for claude.
func parseTranscript(engine, path string) (*trace, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	t := &trace{}
	indexByID := map[string]int{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<29)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if engine == "claude" {
			t.addClaudeLine(line, indexByID)
		} else {
			t.addPiLine(line, indexByID)
		}
	}
	if engine == "claude" && !t.sawResult && t.EngineError == "" {
		t.EngineError = "no result line in the transcript"
	}
	return t, scanner.Err()
}

func (t *trace) addCall(call toolCall, args map[string]any, indexByID map[string]int) {
	if raw, err := json.Marshal(args); err == nil {
		call.Input = string(raw)
	}
	for _, field := range []string{"command", "cmd"} {
		if s, ok := args[field].(string); ok {
			call.Command = s
		}
	}
	for _, field := range []string{"path", "file_path", "notebook_path"} {
		if s, ok := args[field].(string); ok {
			call.Path = s
		}
	}
	indexByID[call.ID] = len(t.Calls)
	t.Calls = append(t.Calls, call)
}

func (t *trace) setResult(id, text string) {
	for i := range t.Calls {
		if t.Calls[i].ID == id {
			t.Calls[i].Result = truncate(text, 4000)
			return
		}
	}
}

func (t *trace) addPiLine(line []byte, indexByID map[string]int) {
	var event struct {
		Type       string
		ToolCallID string
		ToolName   string
		Args       map[string]any
		Result     struct{ Content []struct{ Type, Text string } }
		Message    struct {
			Role         string
			Content      json.RawMessage
			StopReason   string
			ErrorMessage string
			Usage        struct {
				Input, Output, CacheRead, CacheWrite int
				Cost                                 struct{ Total float64 }
			}
		}
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	switch event.Type {
	case "tool_execution_start":
		t.addCall(toolCall{ID: event.ToolCallID, Name: event.ToolName}, event.Args, indexByID)
	case "tool_execution_end":
		var texts []string
		for _, c := range event.Result.Content {
			texts = append(texts, c.Text)
		}
		t.setResult(event.ToolCallID, strings.Join(texts, "\n"))
	case "message_end":
		if event.Message.Role != "assistant" {
			return
		}
		usage := event.Message.Usage
		t.Input += usage.Input
		t.Output += usage.Output
		t.CacheRead += usage.CacheRead
		t.CacheWrite += usage.CacheWrite
		t.CostUSD += usage.Cost.Total
		t.Turns++
		t.EngineError = ""
		if event.Message.StopReason == "error" {
			t.EngineError = "assistant stopped with an error: " + truncate(event.Message.ErrorMessage, 300)
		}
		var blocks []struct{ Type, Text string }
		if json.Unmarshal(event.Message.Content, &blocks) == nil {
			var texts []string
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					texts = append(texts, b.Text)
				}
			}
			if len(texts) > 0 {
				t.FinalText = strings.Join(texts, "\n")
			}
		}
	}
}

func (t *trace) addClaudeLine(line []byte, indexByID map[string]int) {
	var event struct {
		Type, Subtype string
		IsError       bool    `json:"is_error"`
		NumTurns      int     `json:"num_turns"`
		TotalCost     float64 `json:"total_cost_usd"`
		Result        string
		Message       struct{ Content json.RawMessage }
	}
	// claude's usage keys are snake_case with different names than ours.
	var usage struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	switch event.Type {
	case "assistant":
		var blocks []struct {
			Type  string
			ID    string
			Name  string
			Input map[string]any
			Text  string
		}
		if json.Unmarshal(event.Message.Content, &blocks) != nil {
			return
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				t.addCall(toolCall{ID: b.ID, Name: b.Name}, b.Input, indexByID)
			}
		}
	case "user":
		var blocks []struct {
			Type      string
			ToolUseID string `json:"tool_use_id"`
			Content   json.RawMessage
		}
		if json.Unmarshal(event.Message.Content, &blocks) != nil {
			return
		}
		for _, b := range blocks {
			if b.Type != "tool_result" {
				continue
			}
			var text string
			var parts []struct{ Type, Text string }
			if json.Unmarshal(b.Content, &text) != nil && json.Unmarshal(b.Content, &parts) == nil {
				var texts []string
				for _, p := range parts {
					texts = append(texts, p.Text)
				}
				text = strings.Join(texts, "\n")
			}
			t.setResult(b.ToolUseID, text)
		}
	case "result":
		t.sawResult = true
		_ = json.Unmarshal(line, &usage)
		t.Turns, t.CostUSD, t.FinalText = event.NumTurns, event.TotalCost, event.Result
		t.Input, t.Output = usage.Usage.InputTokens, usage.Usage.OutputTokens
		t.CacheRead, t.CacheWrite = usage.Usage.CacheReadInputTokens, usage.Usage.CacheCreationInputTokens
		// Running out of turns is the run's outcome, not an engine failure.
		if (event.IsError || strings.HasPrefix(event.Subtype, "error")) && event.Subtype != "error_max_turns" {
			t.EngineError = fmt.Sprintf("%s: %s", event.Subtype, truncate(event.Result, 300))
		}
	}
}

// editsFiles reports whether a tool call is the engine's file-editing tool.
func editsFiles(name string) bool {
	switch strings.ToLower(name) {
	case "edit", "write", "multiedit", "notebookedit", "str_replace_editor":
		return true
	}
	return false
}

func isShell(name string) bool {
	return strings.EqualFold(name, "bash")
}
