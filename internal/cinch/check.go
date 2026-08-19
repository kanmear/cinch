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
}

func scanErrorFinding(check, scope string, err error) []Finding {
	return []Finding{{
		Check: check, Level: "error", File: scope, Line: 1,
		Message: "failed to scan: " + err.Error(),
	}}
}

func CmdCheck(msgFile string) int {
	return runChecks(msgFile, true)
}

func hooksCheck(msgFile string) int {
	return runChecks(msgFile, false)
}

func runChecks(msgFile string, includeIdentity bool) int {
	docsRoot, err := ResolveDocsRoot(".")
	if err != nil {
		return output.Fail("check", err)
	}

	type namedResult struct {
		name string
		res  checkResult
	}

	ch := make(chan namedResult, 6)
	launch := func(name string, fn func() checkResult) {
		go func() {
			ch <- namedResult{name: name, res: fn()}
		}()
	}

	launch("links", func() checkResult { return checkResult{Findings: checkLinks(docsRoot)} })
	launch("rules", func() checkResult { return checkResult{Findings: checkRules(docsRoot, ".")} })
	if includeIdentity {
		launch("identity", func() checkResult { return checkIdentity(docsRoot, ".") })
	} else {
		launch("identity", func() checkResult {
			return checkResult{NoOp: "HEAD^ vs HEAD lags one commit in pre-commit/commit-msg; run 'cinch check' in CI"}
		})
	}
	launch("generated", func() checkResult { return checkGenerated(".") })
	launch("commit", func() checkResult { return checkCommit(".", msgFile) })
	launch("core", func() checkResult { return checkPin(".", Version) })

	var findings []Finding
	for range 6 {
		r := <-ch
		findings = append(findings, r.res.Findings...)
		output.CheckStatus(r.name, len(r.res.Findings), r.res.NoOp)
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
