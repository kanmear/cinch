package cinch

import (
	"encoding/json"
	"os"
	"strings"

	"cinch/internal/mdscan"
	"cinch/internal/output"
)

// ruleItemJSON, markerLocJSON, docFrontmatterJSON, and rulesInventoryJSON
// are the --json wire shapes for ruleItem/markerLoc/docFrontmatter, since
// those internal types stay unexported per convention and encoding/json
// can't see unexported fields.
type ruleItemJSON struct {
	ID           string          `json:"id"`
	File         string          `json:"file"`
	Line         int             `json:"line"`
	Text         string          `json:"text"`
	HasIgnore    bool            `json:"hasIgnore"`
	IgnoreReason string          `json:"ignoreReason,omitempty"`
	IgnoreLine   int             `json:"ignoreLine,omitempty"`
	Markers      []markerLocJSON `json:"markers"`
}

type markerLocJSON struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type docFrontmatterJSON struct {
	File       string   `json:"file"`
	Owns       []string `json:"owns,omitempty"`
	RulePrefix string   `json:"rulePrefix,omitempty"`
}

type rulesInventoryJSON struct {
	Rules []ruleItemJSON       `json:"rules"`
	Docs  []docFrontmatterJSON `json:"docs"`
}

// buildRulesInventory includes duplicate rule IDs rather than deduping —
// duplication is itself the signal the rules check reports on.
func buildRulesInventory(repoRoot, docsRoot string, options markerScanOptions) (rulesInventoryJSON, error) {
	items, err := scanRuleDocs(docsRoot)
	if err != nil {
		return rulesInventoryJSON{}, err
	}
	markers, _, err := scanRuleMarkers(workingTreeRoots(repoRoot), docsRoot, options)
	if err != nil {
		return rulesInventoryJSON{}, err
	}
	docs, err := scanDocFrontmatter(docsRoot)
	if err != nil {
		return rulesInventoryJSON{}, err
	}

	rules := make([]ruleItemJSON, len(items))
	for i, it := range items {
		var ml []markerLocJSON
		for _, loc := range markers[it.id] {
			ml = append(ml, markerLocJSON{File: loc.file, Line: loc.line})
		}
		rules[i] = ruleItemJSON{
			ID: it.id, File: it.file, Line: it.line, Text: it.text,
			HasIgnore: it.hasIgnore, IgnoreReason: it.ignoreReason, IgnoreLine: it.ignoreLine,
			Markers: ml,
		}
	}

	return rulesInventoryJSON{Rules: rules, Docs: docs}, nil
}

// scanDocFrontmatter returns an entry only for docs that declare owns:
// and/or rule_prefix: — most docs have neither and are silently excluded.
func scanDocFrontmatter(docsRoot string) ([]docFrontmatterJSON, error) {
	return mdscan.Collect(docsRoot, func(path string) ([]docFrontmatterJSON, error) {
		fm, err := parseFrontmatterFile(path)
		if err != nil {
			return nil, err
		}
		if len(fm.Owns) == 0 && fm.RulePrefix == "" {
			return nil, nil
		}
		warnGlobLikeOwns(path, fm.Owns)
		return []docFrontmatterJSON{{File: path, Owns: fm.Owns, RulePrefix: fm.RulePrefix}}, nil
	})
}

// warnGlobLikeOwns nudges a doc author who wrote glob syntax in owns: —
// matching is prefix-based (see ownsMatches in impact.go), so a literal *
// or ? will never match a real path. This is advisory, not a finding: it
// doesn't fail cinch check, it just avoids a silent no-op.
func warnGlobLikeOwns(docPath string, owns []string) {
	for _, entry := range owns {
		if strings.ContainsAny(entry, "*?") {
			output.Skip("rules", docPath+": owns: entry "+entry+" looks like a glob, but owns: matching is prefix-only")
		}
	}
}

func CmdRules(root string, jsonOut bool) int {
	docsRoot, err := ResolveDocsRoot(root)
	if err != nil {
		return output.Fail("rules", err)
	}
	if !jsonOut {
		return output.UsageError("rules: requires --json (no bare-text mode; see 'cinch ignores' or 'cinch check')")
	}
	m, err := loadManifestOptional(root)
	if err != nil {
		return output.Fail("rules", err)
	}
	inv, err := buildRulesInventory(root, docsRoot, markerScanOptionsFor(m))
	if err != nil {
		return output.Fail("rules", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(inv); err != nil {
		return output.Fail("rules", err)
	}
	return 0
}
