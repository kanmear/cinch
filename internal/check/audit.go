package check

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cinch/internal/manifest"
)

// C15 — the coverage audit report must carry machine-verifiable evidence.
// Every row quotes the rule's text verbatim and, for rows citing a test
// (✅ covered or ⚠️ TENSION), the exact assertion line from that test; both
// quotes must be non-empty and appear as substrings in their source files,
// and every closure rule must have a row — a report with no rows is itself a
// vacuous artifact. The report is a committed artifact at the
// manifest-declared `paths.audit`; undeclared, the check skips (absence is a
// declaration, C5/C12 semantics) and C4 covers existence. Error severity: a
// row that cannot back its claim is the D027 false-checkmark class.
//
// The check is model-tier-independent by construction — it never asks a model
// anything, so a 27B report and a frontier report pass or fail the same way
// (D081: the pi vacuous row — no rule text, no test name, ✅ — is the recorded
// failure this exists for). Honest scope: a well-quoted link that is
// semantically wrong passes C15; the content question stays the semantic pass
// (check-rules step 3). Lateral under D009/D079: the report is an authored
// artifact that must agree with its sources.

var auditIDRe = regexp.MustCompile(`^[^A-Z0-9]*([A-Z0-9]+-[0-9]+)`)

type auditRow struct {
	section   string // heading the row sat under (unused beyond messages)
	id        string
	ruleQuote string
	status    string
	file      string
	test      string
	assertion string
	malformed bool // row did not split into the 7 canonical columns
}

func checkAuditReport(root string, m *manifest.Manifest, r *report) {
	rel := m.Vars["paths.audit"]
	if rel == "" {
		return
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return // C4 already reported it
	}
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		return
	}
	dir := filepath.Join(root, bdir)

	domains := map[string]bool{}
	bodies := map[string]string{} // rule ID -> full numbered-item text
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "overview.md" {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		domains[rel] = true
		text := readFile(p)
		for id := range parseDomainDoc(text).rules {
			bodies[id] = ruleItemBody(text, id)
		}
		return nil
	})

	rows := parseAuditReport(string(b), domains)
	for _, row := range rows {
		checkAuditRow(root, m, r, row, bodies)
	}

	// Completeness: every closure rule must have a row. A report with no
	// rows at all is a vacuous artifact — the pi probe's format-deviant
	// report passed every per-row check by having none. A report missing
	// rows for some rules is stale (C2-staleness class): the audit snapshot
	// predates them. Error either way; the executor re-audits.
	covered := map[string]bool{}
	for _, row := range rows {
		if !row.malformed {
			covered[row.id] = true
		}
	}
	var missing []string
	for id := range bodies {
		if !covered[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		if len(covered) == 0 {
			r.errf("C15", "%s has no coverage rows — expected one row per rule in %s/*.md, in the canonical table format", rel, bdir)
		} else {
			r.errf("C15", "%s is missing coverage rows for: %s — re-audit", rel, strings.Join(missing, ", "))
		}
	}
}

func checkAuditRow(root string, m *manifest.Manifest, r *report, row auditRow, bodies map[string]string) {
	where := row.section + " / " + row.id

	if row.malformed {
		r.errf("C15", "audit row for %s (%s) does not have the 7 canonical columns — Rule | Rule text (verbatim) | Testable | Covered | Test file | Test | Assertion (verbatim)", row.id, row.section)
		return
	}

	body, ok := bodies[row.id]
	if !ok {
		r.errf("C15", "audit row for %s names no rule in %s/*.md", row.id, m.Vars["paths.domain"])
		return
	}

	quote := strings.TrimSpace(strings.Trim(row.ruleQuote, "`"))
	if quote == "" {
		r.errf("C15", "audit row %s quotes no rule text — copy it verbatim from the rule's doc", where)
	} else if !containsQuote(body, quote) {
		r.errf("C15", "audit row %s rule text %q is not verbatim in the rule's doc", where, quote)
	}

	switch strings.ToUpper(row.status) {
	case "✅", "✔️":
	case "⚠️ TENSION", "TENSION":
	case "⚠️ MISSING", "MISSING":
	case "N/A":
	default:
		r.errf("C15", "audit row %s has unknown status %q — use ✅ | ⚠️ TENSION | ⚠️ MISSING | N/A", where, row.status)
	}

	covered := strings.Contains(row.status, "✅") || strings.Contains(row.status, "✔️")
	tension := strings.Contains(strings.ToUpper(row.status), "TENSION")
	if !covered && !tension {
		return // MISSING and N/A rows require no test evidence
	}

	file := strings.Trim(strings.TrimSpace(row.file), "`")
	name := strings.Trim(strings.TrimSpace(row.test), "`")
	assert := strings.Trim(strings.TrimSpace(row.assertion), "`")

	content := resolveTestFile(root, m, file)
	if content == nil {
		r.errf("C15", "audit row %s names test file %q, which does not exist", where, file)
		return
	}
	if name == "" {
		r.errf("C15", "audit row %s names no test function", where)
	} else if !strings.Contains(*content, name) {
		r.errf("C15", "audit row %s test %q does not appear in %s", where, name, file)
	}
	if assert == "" {
		r.errf("C15", "audit row %s quotes no assertion line — copy it verbatim from %s", where, file)
	} else if !strings.Contains(*content, assert) {
		r.errf("C15", "audit row %s assertion %q is not verbatim in %s", where, assert, file)
	}
}

// parseAuditReport extracts the coverage-table rows of a report. Only rows
// under a `## <domain>.md` heading naming a real domain doc are audit rows;
// everything else (closure commands, findings, summary tables) is skipped.
// The first cell of a row must be a rule ID — header and separator rows fall
// out naturally. Cells are split on `|` outside backtick runs, so rule texts
// and assertion lines that quote backticks survive. A backtick inside a
// backtick-wrapped cell is markdown-escaped as `\`` in the report; the escape
// is the faithful copy, so it is unescaped before comparison.
func parseAuditReport(text string, domains map[string]bool) []auditRow {
	var rows []auditRow
	section := ""
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(t, "## ") {
			section = strings.TrimPrefix(t, "## ")
			continue
		}
		if !strings.HasPrefix(t, "|") || !domains[section] {
			continue
		}
		cells := splitCells(t)
		id := ""
		if len(cells) > 0 {
			id = auditID(cells[0])
		}
		if id == "" {
			continue
		}
		if len(cells) != 7 {
			rows = append(rows, auditRow{section: section, id: id, malformed: true})
			continue
		}
		rows = append(rows, auditRow{
			section:   section,
			id:        id,
			ruleQuote: unescapeTicks(cells[1]),
			status:    unescapeTicks(cells[3]),
			file:      unescapeTicks(cells[4]),
			test:      unescapeTicks(cells[5]),
			assertion: unescapeTicks(cells[6]),
		})
	}
	return rows
}

func auditID(cell string) string {
	m := auditIDRe.FindStringSubmatch(strings.TrimSpace(strings.Trim(cell, "`*")))
	if m == nil {
		return ""
	}
	return m[1]
}

// unescapeTicks replaces the markdown-escaped form of a backtick inside a
// code-span cell with the literal backtick.
func unescapeTicks(s string) string {
	return strings.ReplaceAll(s, "\\`", "`")
}

func splitCells(line string) []string {
	line = strings.TrimSpace(line)
	leading, trailing := strings.HasPrefix(line, "|"), strings.HasSuffix(line, "|")
	var cells []string
	var cur strings.Builder
	inTick := false
	for _, ch := range line {
		switch {
		case ch == '`':
			inTick = !inTick
			cur.WriteRune(ch)
		case ch == '|' && !inTick:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(ch)
		}
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	// The surrounding pipes produce a leading and a trailing empty cell —
	// drop exactly those, so interior empty cells (a vacuous row's empty
	// quote) survive as real cells.
	if leading {
		cells = cells[1:]
	}
	if trailing {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// ruleItemBody returns the full text of one numbered rule item — the opening
// line plus every continuation line up to the next numbered item or heading —
// so a verbatim quote may be a fragment of the whole item (multi-line rules
// like cinch's own HARNESS items quote a single line; cells cannot span lines).
func ruleItemBody(text, id string) string {
	var buf []string
	open := false
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := ruleItemRe.FindStringSubmatch(line); m != nil {
			if open {
				break
			}
			if m[2] == id {
				open = true
				buf = append(buf, strings.TrimSpace(m[3]))
			}
			continue
		}
		if open {
			if strings.HasPrefix(t, "#") {
				break
			}
			buf = append(buf, t)
		}
	}
	return strings.Join(buf, "\n")
}

// containsQuote reports whether quote appears in body, also tolerating
// indentation drift: continuation lines in domain docs carry leading
// whitespace the report's single-line cell cannot.
func containsQuote(body, quote string) bool {
	if strings.Contains(body, quote) {
		return true
	}
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Contains(strings.Join(lines, "\n"), quote)
}

// resolveTestFile reads a report-named test file, resolving the name from the
// repo root or under any declared paths.tests.* directory. Returns nil when
// no candidate exists (C4-style existence; the row then errors).
func resolveTestFile(root string, m *manifest.Manifest, file string) *string {
	bases := []string{root}
	for k, v := range m.PathValues() {
		if strings.HasPrefix(k, "paths.tests.") {
			bases = append(bases, filepath.Join(root, v))
		}
	}
	for _, base := range bases {
		b, err := os.ReadFile(filepath.Join(base, file))
		if err == nil {
			s := string(b)
			return &s
		}
	}
	return nil
}
