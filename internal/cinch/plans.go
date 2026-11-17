package cinch

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

const (
	plansLifecycleKey = "plans.lifecycle"
	plansEnforceValue = "enforce"

	// statusLineWindow is how many lines from the top of a plan are searched
	// for its status line; a per-task `**Status:**` further down isn't the
	// plan's.
	statusLineWindow = 15
	statusLinePrefix = "**Status:**"
)

var (
	livePlanStatuses     = []string{"planned", "in-progress"}
	archivedPlanStatuses = []string{"complete", "cut"}
)

// parseStatusLine finds a plan's status in the first statusLineWindow lines of
// content: the first line starting at column 0 with `**Status:**`. word is the
// text after the marker up to the first whitespace (empty if nothing follows),
// line is 1-based, and found is false when no line in the window qualifies.
func parseStatusLine(content string) (word string, line int, found bool) {
	for i, text := range strings.SplitN(content, "\n", statusLineWindow+1) {
		if i == statusLineWindow {
			break
		}
		rest, ok := strings.CutPrefix(text, statusLinePrefix)
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			word = fields[0]
		}
		return word, i + 1, true
	}
	return "", 0, false
}

// plansAllowedStatuses is the status words a plan directly in the docs-relative
// folder may carry; ok is false for a folder the check doesn't cover.
func plansAllowedStatuses(folder string) (allowed []string, ok bool) {
	switch folder {
	case "plans", "plans/fix":
		return livePlanStatuses, true
	case "plans/archive":
		return archivedPlanStatuses, true
	}
	return nil, false
}

func checkPlans(roots checkRoots, docsRoot string, m *manifest) checkResult {
	mode := manifestVar(m, plansLifecycleKey, "")
	if mode == "" {
		return checkResult{noOp: plansLifecycleKey + " not set — nothing to enforce"}
	}
	if mode != plansEnforceValue {
		return checkResult{findings: []finding{{
			check: "plans", level: "error", file: filepath.Join(roots.repoRoot, manifestPath), line: 1,
			message: plansLifecycleKey + ": unknown value \"" + mode + "\" (the only allowed value is \"" + plansEnforceValue + "\")",
		}}}
	}

	var findings []finding
	checked := 0
	err := roots.walkDocs(docsRoot, func(file string) error {
		folder := path.Dir(filepath.ToSlash(mdscan.RelTo(docsRoot, file)))
		allowed, ok := plansAllowedStatuses(folder)
		if !ok {
			return nil
		}
		data, err := roots.readFile(file)
		if err != nil {
			return err
		}
		checked++
		word, line, found := parseStatusLine(string(data))
		list := strings.Join(allowed, ", ")
		switch {
		case !found:
			findings = append(findings, finding{
				check: "plans", level: "error", file: file, line: 1,
				message: "no **Status:** line in the first 15 lines (allowed in " + folder + "/: " + list + ")",
			})
		case !slices.Contains(allowed, word):
			findings = append(findings, finding{
				check: "plans", level: "error", file: file, line: line,
				message: "status \"" + word + "\" is not allowed in " + folder + "/ (allowed: " + list + ")",
			})
		}
		return nil
	})
	if err != nil {
		findings = append(findings, scanErrorFinding("plans", docsRoot, err)...)
	}
	return checkResult{findings: findings, detail: fmt.Sprintf("(%d %s checked)", checked, output.Plural(checked, "plan"))}
}
