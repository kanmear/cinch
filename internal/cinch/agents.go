package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cinch/internal/output"
)

const (
	agentsFile       = "AGENTS.md"
	claudeFile       = "CLAUDE.md"
	agentsPointerKey = "agents.pointer"

	agentsHeading = "# Agent instructions\n\n"

	// The rule-marker and rule-ID examples here are scanned like any other
	// file outside the docs root (markerRe, parseRuleItems): keep <ID> a
	// placeholder and never put a comment leader before cinch:rule, or init
	// would plant a phantom marker.
	agentPointerTemplate = "## Project docs (cinch)\n" +
		"\n" +
		"This repo's operational docs live in `%s/`. Read them before changing behavior they describe.\n" +
		"\n" +
		"- `cinch index` lists every doc with its title; `cinch workflows` lists the workflows, `cinch workflow <name>` prints one.\n" +
		"- Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.\n" +
		"- Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.\n" +
		"- `cinch check` must pass before you commit.\n"
)

func agentsPointerEnabled(m *manifest) bool {
	value, ok := manifestSetting(m, agentsPointerKey)
	return !ok || !strings.EqualFold(value, "false")
}

// ensureAgentPointer only ever adds content: existing AGENTS.md bytes are
// never rewritten, and a file that already mentions cinch is left alone so a
// consumer's own pointer wins (and a second run is a no-op).
func ensureAgentPointer(repoRoot, docsPath string) error {
	path := filepath.Join(repoRoot, agentsFile)
	block := fmt.Sprintf(agentPointerTemplate, strings.TrimRight(docsPath, "/"))

	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(agentsHeading+block), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", agentsFile, err)
		}
		output.Step("wrote %s", agentsFile)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", agentsFile, err)
	}

	if strings.Contains(strings.ToLower(string(existing)), "cinch") {
		output.Step("%s already mentions cinch — left as-is", agentsFile)
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", agentsFile, err)
	}
	if _, err := f.WriteString(blockSeparator(existing) + block); err != nil {
		_ = f.Close()
		return fmt.Errorf("append %s: %w", agentsFile, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("append %s: %w", agentsFile, err)
	}
	output.Step("added cinch section to %s", agentsFile)
	return nil
}

// blockSeparator returns what must precede an appended block so exactly one
// blank line separates it from the existing content.
func blockSeparator(existing []byte) string {
	text := string(existing)
	switch {
	case strings.TrimSpace(text) == "", strings.HasSuffix(text, "\n\n"):
		return ""
	case strings.HasSuffix(text, "\n"):
		return "\n"
	default:
		return "\n\n"
	}
}

func hintClaudeMd(repoRoot string) {
	data, err := os.ReadFile(filepath.Join(repoRoot, claudeFile))
	if err != nil {
		return
	}
	text := strings.ToLower(string(data))
	if strings.Contains(text, "agents.md") || strings.Contains(text, "cinch") {
		return
	}
	output.Skip("init", "CLAUDE.md doesn't reference AGENTS.md — tools that read CLAUDE.md won't see the cinch section; consider adding '@AGENTS.md' to it")
}
