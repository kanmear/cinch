package cinch

import (
	"fmt"
	"sort"

	"cinch/internal/output"
)

type Finding struct {
	Check   string
	Level   string
	File    string
	Line    int
	Message string
}

type checkResult struct {
	Findings []Finding
	NoOp     string
	Detail   string
}

func scanErrorFinding(check, scope string, err error) []Finding {
	return []Finding{{
		Check: check, Level: "error", File: scope, Line: 1,
		Message: "failed to scan: " + err.Error(),
	}}
}

func CmdCheck(msgFile string) int {
	return runChecks(".", msgFile, true)
}

func hooksCheckStatic(root string) int {
	return runChecks(root, "", false, "links", "rules", "identity", "generated", "core")
}

func hooksCheckCommitMsg(root, msgFile string) int {
	return runChecks(root, msgFile, false, "commit")
}

func runChecks(root, msgFile string, includeIdentity bool, only ...string) int {
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
		name string
		res  checkResult
	}

	ch := make(chan namedResult, 6)
	launched := 0
	launch := func(name string, fn func() checkResult) {
		if !run(name) {
			return
		}
		launched++
		go func() {
			ch <- namedResult{name: name, res: fn()}
		}()
	}

	launch("links", func() checkResult { return linksCheckResult(checkLinks(docsRoot)) })
	launch("rules", func() checkResult { return rulesCheckResult(checkRules(docsRoot, root)) })
	if includeIdentity {
		launch("identity", func() checkResult { return checkIdentity(docsRoot, root) })
	} else {
		launch("identity", func() checkResult {
			return checkResult{NoOp: "HEAD^ vs HEAD lags one commit in pre-commit/commit-msg; run 'cinch check' in CI"}
		})
	}
	launch("generated", func() checkResult { return checkGenerated(root) })
	launch("commit", func() checkResult { return checkCommit(root, msgFile) })
	launch("core", func() checkResult { return checkPin(root, Version) })

	var findings []Finding
	for i := 0; i < launched; i++ {
		r := <-ch
		findings = append(findings, r.res.Findings...)
		output.CheckStatus(r.name, len(r.res.Findings), r.res.NoOp, r.res.Detail)
	}

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})

	for _, f := range findings {
		fmt.Printf("%s %s %s:%d: %s\n", f.Check, output.Level(f.Level), f.File, f.Line, f.Message)
	}

	if len(findings) > 0 {
		return 1
	}
	return 0
}
