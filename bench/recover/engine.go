package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// answerSuffix follows the question in every prompt, verbatim.
const answerSuffix = "Answer in at most five sentences. State the specific behavior (numbers, conditions, who may do what). Cite the file and line you based it on. If the repository does not settle it, say so."

// buildPrompt is the question, then the answer instructions.
func buildPrompt(q string) string {
	return strings.TrimSpace(q) + "\n\n" + answerSuffix
}

// trace is what a transcript says happened.
type trace struct {
	ToolCalls                       int
	Turns, Input, Output, CacheRead int
	CacheWrite                      int
	CostUSD                         float64
	FinalText                       string
	EngineError                     string
}

// agentRun is how the agent process ended.
type agentRun struct {
	ExitCode int
	TimedOut bool
	WallMS   int64
}

// runAgent starts the engine in a sandbox over repo with the prompt, verbatim,
// and writes its raw stdout to transcript.jsonl and its stderr to stderr.txt.
// A timeout kills the sandbox's whole process group; the run is recorded as it
// stands, never retried. The fake engine runs a script in the same sandbox in
// place of a model, with the prompt as its first argument.
func (h *harness) runAgent(ctx context.Context, repo, rundir, prompt string) (agentRun, error) {
	timeout, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	spec := sandboxSpec{repo: repo, rundir: rundir, engine: h.engine}
	var argv []string
	switch h.engine {
	case "pi":
		// Read-only tools: the census asks, it does not ask for changes.
		argv = []string{h.sandbox.piNode, h.sandbox.piCLI, "-p", "--mode", "json",
			"--provider", h.provider, "--model", h.model, "--thinking", h.thinking,
			"--no-session", "--offline", "--approve", "-ne", "-ns", "-np",
			"-t", "read,bash,grep,find,ls", "--", prompt}
	case "fake":
		argv = []string{"/bin/bash", sandboxScript, prompt}
	default:
		return agentRun{}, fmt.Errorf("engine %q runs no process", h.engine)
	}
	built, err := h.sandboxCmd(spec, argv...)
	if err != nil {
		return agentRun{}, err
	}
	cmd := exec.CommandContext(timeout, built.Path, built.Args[1:]...)
	cmd.Env = built.Env
	// The agent spawns its own children; kill the whole group.
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
	ended := agentRun{WallMS: time.Since(start).Milliseconds()}
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

// parseTranscript reads an engine's raw output into usage and the final text.
// For pi, usage is summed over assistant message_end events (the other events
// repeat them) and the answer is the last assistant message's text. The fake
// engine's whole stdout is its answer.
func parseTranscript(engine, path string) (*trace, error) {
	if engine == "fake" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return &trace{FinalText: strings.TrimSpace(string(data)), Turns: 1}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	t := &trace{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<29)
	for scanner.Scan() {
		if line := scanner.Bytes(); len(bytes.TrimSpace(line)) > 0 {
			t.addPiLine(line)
		}
	}
	t.FinalText = strings.TrimSpace(t.FinalText)
	return t, scanner.Err()
}

func (t *trace) addPiLine(line []byte) {
	var event struct {
		Type    string
		Message struct {
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
		t.ToolCalls++
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
			// The answer is the last assistant message's text; a run cut off
			// mid-tool-call has none.
			t.FinalText = strings.Join(texts, "\n")
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…"
}
