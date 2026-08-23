package cinch

import (
	"fmt"
	"sort"

	"cinch/internal/output"
)

type finding struct {
	check   string
	level   string
	file    string
	line    int
	message string
}

type checkResult struct {
	findings []finding
	noOp     string
	detail   string
}

func scanErrorFinding(check, scope string, err error) []finding {
	return []finding{{
		check: check, level: "error", file: scope, line: 1,
		message: "failed to scan: " + err.Error(),
	}}
}

func CmdCheck(messageFile string) int {
	return runChecks(".", messageFile, true)
}

func preCommitChecks(root string) int {
	return runChecks(root, "", false, "links", "rules", "retirement", "generated", "core")
}

func commitMsgChecks(root, messageFile string) int {
	return runChecks(root, messageFile, false, "commit")
}

func runChecks(root, messageFile string, includeRetirement bool, only ...string) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("check", err)
	}

	onlySet := make(map[string]bool, len(only))
	for _, n := range only {
		onlySet[n] = true
	}
	run := func(name string) bool { return len(only) == 0 || onlySet[name] }

	type namedResult struct {
		name   string
		result checkResult
	}

	results := make(chan namedResult, 7)
	launched := 0
	launch := func(name string, fn func() checkResult) {
		if !run(name) {
			return
		}
		launched++
		go func() {
			results <- namedResult{name: name, result: fn()}
		}()
	}

	launch("links", func() checkResult { return linksCheckResult(checkLinks(docsRoot)) })
	launch("rules", func() checkResult { return rulesCheckResult(checkRules(root, docsRoot)) })
	if includeRetirement {
		launch("retirement", func() checkResult { return checkRetirement(root, docsRoot) })
	} else {
		launch("retirement", func() checkResult {
			return checkResult{noOp: "HEAD^ vs HEAD lags one commit in pre-commit/commit-msg; run 'cinch check' in CI"}
		})
	}
	launch("generated", func() checkResult { return checkGenerated(root) })
	launch("commit", func() checkResult { return checkCommit(root, messageFile) })
	launch("core", func() checkResult { return checkPin(root, Version) })
	launch("hooks", func() checkResult { return checkHooks(root) })

	var findings []finding
	for i := 0; i < launched; i++ {
		r := <-results
		findings = append(findings, r.result.findings...)
		output.CheckStatus(r.name, len(r.result.findings), r.result.noOp, r.result.detail)
	}

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.check != b.check {
			return a.check < b.check
		}
		if a.file != b.file {
			return a.file < b.file
		}
		return a.line < b.line
	})

	for _, f := range findings {
		fmt.Printf("%s %s %s:%d: %s\n", f.check, output.Level(f.level), f.file, f.line, f.message)
	}

	if len(findings) > 0 {
		return 1
	}
	return 0
}
