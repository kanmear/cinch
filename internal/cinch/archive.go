package cinch

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

const (
	plansFolder        = "plans"
	plansArchiveFolder = "plans/archive"
)

func CmdPlanArchive(repoRoot string, files []string) int {
	docsRoot, err := ResolveDocsRoot(repoRoot)
	if err != nil {
		return output.Fail("plan archive", err)
	}
	reports, err := archivePlans(repoRoot, docsRoot, files)
	var oldPaths []string
	for _, report := range reports {
		printMoveReport(report)
		oldPaths = append(oldPaths, report.source)
	}
	if len(oldPaths) > 0 {
		printMoveReminder(oldPaths)
	}
	if err != nil {
		return output.Fail("plan archive", err)
	}
	return 0
}

// archivePlans validates every file before moving any, so a refusal anywhere
// in the list leaves the tree untouched. The moves then run one after another:
// each sees the link rewrites of the one before.
func archivePlans(repoRoot, docsRoot string, files []string) ([]moveReport, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	absDocs, err := filepath.Abs(docsRoot)
	if err != nil {
		return nil, err
	}

	targets := make([]string, len(files))
	seen := make(map[string]string)
	for i, file := range files {
		absFile := resolveRepoPath(absRepo, file)
		rel := filepath.ToSlash(mdscan.RelTo(absDocs, absFile))
		if folder := path.Dir(rel); folder != plansFolder || !strings.HasSuffix(rel, ".md") {
			if !strings.HasSuffix(absFile, ".md") {
				return nil, fmt.Errorf("%s: not a .md file", file)
			}
			return nil, fmt.Errorf("%s: not directly in %s/%s/ (plans/fix/, plans/archive/ and deeper folders are not archived)",
				file, repoRelative(absRepo, absDocs), plansFolder)
		}
		base := path.Base(rel)
		if first, dup := seen[base]; dup {
			return nil, fmt.Errorf("%s: %s would be archived twice (same name as %s)", file, base, first)
		}
		seen[base] = file

		target := filepath.Join(absDocs, filepath.FromSlash(plansArchiveFolder), base)
		if _, _, _, _, err := validateMove(repoRoot, docsRoot, file, repoRelative(absRepo, target)); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(absFile)
		if err != nil {
			return nil, err
		}
		word, _, found := parseStatusLine(string(data))
		switch {
		case !found:
			return nil, fmt.Errorf("%s: no **Status:** line in the first %d lines", file, statusLineWindow)
		case !slices.Contains(archivedPlanStatuses, word):
			return nil, fmt.Errorf("%s: status %q is not %s", file, word, strings.Join(archivedPlanStatuses, " or "))
		}
		targets[i] = repoRelative(absRepo, target)
	}

	var reports []moveReport
	for i, file := range files {
		report, err := moveFile(repoRoot, docsRoot, file, targets[i])
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
