package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// C8 — the rule→marker closure (D027, D057, D064). Every rule ID in a
// `domain/*.md` doc (not overview.md — philosophies get no IDs, D027) must be
// enforced by at least one `// cinch:rule <ID>` marker; every marker must
// resolve to a real rule. A rule that is untestable by design declares
// `cinch:ignore <ID>` in its doc and drops out of the coverage question —
// `cinch ignores` lists the inventory. Forward (rule without marker) warns:
// coverage is a question, and cinch's own R1-class rules are untestable by
// design. Reverse (marker without rule) errors: a stale marker is a false
// checkmark, the worst failure mode in the loop (D027).
//
// IDs are author-assigned, permanent, never reused, prefixed from the doc's
// rule_prefix front-matter (D027). A doc that declares rules but no prefix is
// pre-migration and errors. Numbers on list items never change — the ID is
// the identity; a retired rule keeps its ID with a retirement note (R5).

var (
	ruleItemRe   = regexp.MustCompile(`^\s*\d+\.\s+(\*\*([A-Z0-9]+-[0-9]+)\*\*)?\s*(.*)$`)
	ruleMarkerRe = regexp.MustCompile(`//\s*cinch:rule\s+([A-Z0-9]+-[0-9]+)`)
	ruleIgnoreRe = regexp.MustCompile(`cinch:ignore\s+([A-Z0-9]+-[0-9]+)`)
)

type domainRule struct {
	id   string
	doc  string // repo-relative path
	text string // opening text of the numbered item
}

// parseDomainDoc parses one domain doc: its rule_prefix, the rule IDs its
// numbered items carry (id -> opening text), the IDs declared cinch:ignore,
// and the opening text of numbered items carrying no ID (pre-migration).
// Numbered items inside fenced blocks are examples, not rules (D046, same as
// C7/C11).
func parseDomainDoc(text string) domainDoc {
	dd := domainDoc{rules: map[string]string{}}
	dd.prefix = rulePrefix(text)
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := ruleItemRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[2] == "" {
			dd.noID = append(dd.noID, strings.TrimSpace(m[3]))
			continue
		}
		dd.rules[m[2]] = strings.TrimSpace(m[3])
	}
	for _, im := range ruleIgnoreRe.FindAllStringSubmatch(text, -1) {
		dd.ignores = append(dd.ignores, im[1])
	}
	return dd
}

type domainDoc struct {
	prefix  string
	rules   map[string]string
	ignores []string
	noID    []string
}

// rulePrefix reads rule_prefix from the doc's YAML front-matter block.
func rulePrefix(text string) string {
	end := strings.Index(text, "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(text[:end], "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "rule_prefix:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// C8 — see the comment at the top of this file.
func checkRules(root string, m *Manifest, r *report) {
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		return // no domain layer declared (C5 same semantics)
	}
	dir := filepath.Join(root, bdir)

	rules := map[string]domainRule{}
	ignored := map[string]string{} // id -> repo-relative doc path

	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "overview.md" {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		dd := parseDomainDoc(readFile(p))

		if len(dd.rules) > 0 && dd.prefix == "" {
			r.errf("C8", "domain/%s declares rules but no rule_prefix front-matter", rel)
		}
		for _, body := range dd.noID {
			r.errf("C8", "domain/%s rule %q has no rule ID — expected **<prefix>-NNN** from rule_prefix front-matter", rel, body)
		}
		for id, text := range dd.rules {
			pfx, _, _ := strings.Cut(id, "-")
			if dd.prefix != "" && pfx != dd.prefix {
				r.errf("C8", "domain/%s rule %s uses prefix %q, doc declares rule_prefix %q", rel, id, pfx, dd.prefix)
			}
			rules[id] = domainRule{id: id, doc: filepath.Join(bdir, rel), text: text}
		}
		for _, id := range dd.ignores {
			if _, ok := dd.rules[id]; !ok {
				r.errf("C8", "domain/%s declares cinch:ignore %s, but that doc has no rule %s", rel, id, id)
				continue
			}
			ignored[id] = filepath.Join(bdir, rel)
		}
		return nil
	})

	markers := scanRuleMarkers(root)

	for id := range markers {
		if _, ok := rules[id]; !ok {
			r.errf("C8", "// cinch:rule %s does not resolve to a rule in %s/*.md", id, bdir)
		}
	}
	for id, rule := range rules {
		switch {
		case markers[id] && ignored[id] != "":
			r.errf("C8", "rule %s is both marked and cinch:ignore'd — one truth", id)
		case markers[id] || ignored[id] != "":
		default:
			r.warnf("C8", "rule %s (%s) has no // cinch:rule marker — add one above its enforcing test, or declare cinch:ignore", id, rule.doc)
		}
	}
}

// scanRuleMarkers finds every `// cinch:rule <ID>` token in the source tree.
// The marker is a comment token, greppable in any language — core never parses
// project source (D003). Documentation surfaces are not source and are skipped:
// `.agent/` (rendered workflows and authored harness docs), `templates/` and
// `docs/` (prose that teaches the marker syntax), and `decisions.jsonl` (the
// append-only record — D027's own example token lives there permanently and
// must never resolve). `.git/` and `bin/` are artifacts. Binary files are
// never read whole.
func scanRuleMarkers(root string) map[string]bool {
	found := map[string]bool{}
	skipDir := map[string]bool{".git": true, "bin": true, "docs": true, ".agent": true, "templates": true}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "decisions.jsonl" {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() > 4<<20 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, m := range ruleMarkerRe.FindAllStringSubmatch(string(b), -1) {
			found[m[1]] = true
		}
		return nil
	})
	return found
}

// cmdIgnores lists every rule declared cinch:ignore — the enumerable inventory
// of untested-but-legitimate rules for the check-rules and optimize-docs
// audits.
func cmdIgnores(root string) error {
	m, err := loadManifest(root)
	if err != nil {
		return err
	}
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		return nil
	}
	dir := filepath.Join(root, bdir)

	type row struct{ id, doc, text string }
	var rows []row
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "overview.md" {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		dd := parseDomainDoc(readFile(p))
		for _, id := range dd.ignores {
			rows = append(rows, row{id, filepath.Join(bdir, rel), dd.rules[id]})
		}
		return nil
	})

	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	for _, row := range rows {
		fmt.Printf("%s  %s  %s\n", row.id, row.doc, row.text)
	}
	return nil
}
