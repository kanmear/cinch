package cinch

import (
	"fmt"
	"path/filepath"
	"sort"

	"cinch/internal/output"
)

const pathsDocsKey = "paths.docs"
const defaultDocsPath = ".docs"

func docsPathValue(m *Manifest) string {
	if m != nil {
		if v, ok := m.Vars[pathsDocsKey]; ok && v != "" {
			return v
		}
	}
	return defaultDocsPath
}

func ResolveDocsRoot(root string) (string, error) {
	m, err := loadManifestOptional(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, docsPathValue(m)), nil
}

type Finding struct {
	Check   string
	Level   string
	File    string
	Line    int
	Message string
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

	var findings []Finding

	linksFindings := checkLinks(docsRoot)
	findings = append(findings, linksFindings...)
	output.CheckStatus("links", len(linksFindings), "")

	rulesFindings := checkRules(docsRoot, ".")
	findings = append(findings, rulesFindings...)
	output.CheckStatus("rules", len(rulesFindings), "")

	if includeIdentity {
		identity := checkIdentity(docsRoot, ".")
		findings = append(findings, identity.Findings...)
		output.CheckStatus("identity", len(identity.Findings), identity.NoOp)
	} else {
		output.Skip("check", "identity", "HEAD^ vs HEAD lags one commit in pre-commit/commit-msg; run 'cinch check' in CI")
	}

	generated := checkGenerated(".")
	findings = append(findings, generated.Findings...)
	output.CheckStatus("generated", len(generated.Findings), generated.NoOp)

	commit := checkCommit(".", msgFile)
	findings = append(findings, commit.Findings...)
	output.CheckStatus("commit", len(commit.Findings), commit.NoOp)

	pin := checkPin(".", Version)
	findings = append(findings, pin.Findings...)
	output.CheckStatus("core", len(pin.Findings), pin.NoOp)

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
