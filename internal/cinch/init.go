package cinch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cinch/internal/gitutil"
	"cinch/internal/output"
)

type initAnswers struct {
	docsPath      string
	hooksPath     string
	commitPattern string
	require       string
	preCommits    []hookEntry
}

type hookEntry struct {
	name string
	run  string
	when string
}

func CmdInit(root string) int {
	if !ManifestExists(root) {
		var answers initAnswers
		if output.IsInteractiveStdin(os.Stdin) {
			answers = askInit()
		} else {
			answers = initAnswers{docsPath: defaultDocsPath, hooksPath: defaultHooksPath}
		}
		if err := writeInitManifest(root, answers); err != nil {
			return output.Fail("init", err)
		}
		output.Step("wrote %s", manifestPath)
	} else {
		output.Step("%s already exists — left as-is", manifestPath)
	}

	m, err := loadManifest(root)
	if err != nil {
		return output.Fail("init", err)
	}

	if code := CmdRender(root); code != 0 {
		return code
	}

	if gitutil.IsRepo(root) {
		hooksDirectory := hooksPathValue(m)
		if existing, err := gitutil.Output(root, "config", "--get", "core.hooksPath"); err == nil {
			if got := strings.TrimSpace(string(existing)); got != "" && !sameHooksDirectory(root, got, hooksDirectory) {
				return output.Failf("init", "core.hooksPath is already %q (expected %q) — cinch init refuses to overwrite another tool's hook wiring; point paths.hooks at %q or resolve the conflict by hand", got, hooksDirectory, got)
			}
		}
		if out, err := gitutil.CombinedOutput(root, "config", "core.hooksPath", hooksDirectory); err != nil {
			return output.Failf("init", "git config core.hooksPath failed: %v\n%s", err, out)
		}
		output.Step("activated hooks: core.hooksPath = %s", hooksDirectory)
	} else {
		output.Skip("hooks", "not a git repository — hook shims generated but not activated")
	}

	if !agentsPointerEnabled(m) {
		output.Skip("init", fmt.Sprintf("%s is false — %s left alone", agentsPointerKey, agentsFile))
		return 0
	}
	if err := ensureAgentPointer(root, manifestVar(m, pathsDocsKey, defaultDocsPath)); err != nil {
		return output.Fail("init", err)
	}
	hintClaudeMd(root)

	return 0
}

func askInit() initAnswers {
	answers := initAnswers{}

	answers.docsPath = askLine("paths.docs (default .docs)", defaultDocsPath)
	answers.hooksPath = askLine("paths.hooks (default .githooks)", defaultHooksPath)

	if output.AskYesNo("enforce a commit message pattern?") {
		answers.commitPattern = askLine("commit.pattern", "")
	}

	if Version != "dev" && output.AskYesNo(fmt.Sprintf("require cinch %s?", Version)) {
		answers.require = Version
	}

	for {
		name := askLine("pre-commit hook name (blank to finish)", "")
		if name == "" {
			break
		}
		run := askLine("  run", "")
		if run == "" {
			continue
		}
		entry := hookEntry{name: name, run: run}
		if when := askLine("  when (comma-separated paths, blank for always)", ""); when != "" {
			entry.when = when
		}
		answers.preCommits = append(answers.preCommits, entry)
	}

	return answers
}

func askLine(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line := output.ReadLine()
	if line == "" {
		return defaultValue
	}
	return line
}

func writeInitManifest(root string, a initAnswers) error {
	var b strings.Builder

	b.WriteString("paths:\n")
	if a.docsPath == "" {
		a.docsPath = defaultDocsPath
	}
	if a.hooksPath == "" {
		a.hooksPath = defaultHooksPath
	}
	fmt.Fprintf(&b, "  docs: %s\n", a.docsPath)
	fmt.Fprintf(&b, "  hooks: %s\n", a.hooksPath)

	if a.commitPattern != "" {
		fmt.Fprintf(&b, "\ncommit:\n  pattern: '%s'\n", a.commitPattern)
	}
	if a.require != "" {
		fmt.Fprintf(&b, "\nrequire:\n  cinch: %s\n", a.require)
	}
	if len(a.preCommits) > 0 {
		b.WriteString("\nhooks:\n  pre-commit:\n")
		for _, e := range a.preCommits {
			fmt.Fprintf(&b, "    %s:\n      run: %s\n", e.name, e.run)
			if e.when != "" {
				fmt.Fprintf(&b, "      when: [%s]\n", e.when)
			}
		}
	}

	path := filepath.Join(root, manifestPath)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
