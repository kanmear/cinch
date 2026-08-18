package cinch

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cinch/internal/output"
)

type initAnswers struct {
	docsPath   string
	hooksPath  string
	commitPat  string
	require    string
	preCommits []hookEntry
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

	if isGitRepo(root) {
		hooksDir := hooksPathValue(m)
		cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return output.Failf("init", "git config core.hooksPath failed: %v\n%s", err, out)
		}
		output.Step("activated hooks: core.hooksPath = %s", hooksDir)
	} else {
		output.Skip("init", "hooks", "not a git repository — hook shims generated but not activated")
	}

	return 0
}

func askInit() initAnswers {
	answers := initAnswers{}

	answers.docsPath = askLine("paths.docs (default .docs)", defaultDocsPath)
	answers.hooksPath = askLine("paths.hooks (default .githooks)", defaultHooksPath)

	if askYesNo("enforce a commit message pattern?") {
		answers.commitPat = askLine("commit.pattern", "")
	}

	if Version != "dev" && askYesNo(fmt.Sprintf("require cinch %s?", Version)) {
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

func askLine(prompt, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", prompt, def)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line := readLine()
	if strings.TrimSpace(line) == "" {
		return def
	}
	return strings.TrimSpace(line)
}

func askYesNo(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	return strings.ToLower(readLine()) == "y"
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
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

	if a.commitPat != "" {
		fmt.Fprintf(&b, "\ncommit:\n  pattern: '%s'\n", a.commitPat)
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
