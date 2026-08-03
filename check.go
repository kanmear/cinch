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

// Wave one of the checker layer (D030): C1, C2, C3, C4, C5, C9, C11, C12.
// C6/C7 (introspection) and C8 (rule markers) land later.
//
// D009 — the direction rule: only doc-upstream and lateral checks belong here.
// A check that validates a doc's copy of a machine-readable fact is
// doc-downstream; delete the duplication instead of adding the check.

type finding struct {
	code  string
	level string // "error" | "warn"
	msg   string
}

type report struct{ findings []finding }

func (r *report) errf(code, format string, a ...any) {
	r.findings = append(r.findings, finding{code, "error", fmt.Sprintf(format, a...)})
}

func (r *report) warnf(code, format string, a ...any) {
	r.findings = append(r.findings, finding{code, "warn", fmt.Sprintf(format, a...)})
}

func cmdCheck(root string) error {
	r := &report{}

	m, err := loadManifest(root)
	if err != nil {
		r.errf("C3", "%v", err)
		return emit(r)
	}

	checkIndex(root, r)            // C1
	checkRendered(root, m, r)      // C2
	checkCommandIDs(m, r)          // C3
	checkManifestPaths(root, m, r) // C4
	checkDomains(root, m, r)       // C5
	checkPlans(root, r)            // C9
	checkLinks(root, r)            // C11
	checkSeams(m, r)               // C12

	return emit(r)
}

func emit(r *report) error {
	sort.SliceStable(r.findings, func(i, j int) bool { return r.findings[i].code < r.findings[j].code })

	errs := 0
	for _, f := range r.findings {
		if f.level == "error" {
			errs++
		}
		fmt.Printf("%-5s %-5s %s\n", f.code, f.level, f.msg)
	}
	if len(r.findings) == 0 {
		fmt.Println("harness ok")
		return nil
	}
	if errs > 0 {
		return fmt.Errorf("%d error(s), %d warning(s)", errs, len(r.findings)-errs)
	}
	fmt.Printf("\n%d warning(s), no errors\n", len(r.findings))
	return nil
}

// C1 — the committed index must match a fresh generation.
func checkIndex(root string, r *report) {
	want, err := buildIndex(root)
	if err != nil {
		r.errf("C1", "building index: %v", err)
		return
	}
	got, err := os.ReadFile(indexPath(root))
	if err != nil {
		r.errf("C1", ".agent/index.md missing — run `cinch index`")
		return
	}
	if string(got) != want {
		r.errf("C1", ".agent/index.md is stale — run `cinch index`")
	}
}

// C2 — rendered workflows must match a fresh render (staleness), and their body
// hash must match their header (tamper). The two are distinguished so the
// message tells you which happened.
func checkRendered(root string, m *Manifest, r *report) {
	files, err := renderAll(root, m)
	if err != nil {
		r.errf("C2", "%v", err)
		return
	}
	for _, f := range files {
		dst := filepath.Join(workflowsDir(root), f.rel)
		raw, err := os.ReadFile(dst)
		if err != nil {
			r.errf("C2", "workflows/%s not rendered — run `cinch render`", f.rel)
			continue
		}
		hdr, body := splitGenerated(string(raw))
		if hdr == "" {
			r.errf("C2", "workflows/%s has no generated header — is it really generated?", f.rel)
			continue
		}
		switch {
		case headerHash(hdr) != bodyHash(body):
			r.errf("C2", "workflows/%s was hand-edited — revert it and edit templates/%s", f.rel, f.rel)
		case body != f.body:
			r.errf("C2", "workflows/%s is stale against the manifest — run `cinch render`", f.rel)
		}
	}
}

// C3 — every test tier's cmd id resolves under development.commands.
func checkCommandIDs(m *Manifest, r *report) {
	cmds := m.commandKeys()
	tiers := m.testTierCommands()
	for _, tier := range sortedKeys(tiers) {
		cmd := tiers[tier]
		if !cmds[cmd] {
			r.errf("C3", "test tier %q declares cmd %q, which has no key under development.commands", tier, cmd)
		}
	}
}

// C4 — every path binding points at something that exists.
func checkManifestPaths(root string, m *Manifest, r *report) {
	pv := m.pathValues()
	for _, k := range sortedKeys(pv) {
		p := filepath.Join(root, pv[k])
		if _, err := os.Stat(p); err != nil {
			r.errf("C4", "manifest %s -> %s does not exist", k, pv[k])
		}
	}
}

// C5 — the domain list is the filesystem; overview.md must agree with it.
func checkDomains(root string, m *Manifest, r *report) {
	bdir := m.Vars["paths.business"]
	if bdir == "" {
		// No business layer declared. Legitimate — cinch itself has none, and so
		// will plenty of consumers. Absence of the binding is a declaration about
		// the repo's shape, not an omission.
		return
	}
	abs := filepath.Join(root, bdir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return // C4 already reported it
	}
	overview, err := os.ReadFile(filepath.Join(abs, "overview.md"))
	if err != nil {
		r.errf("C5", "%s/overview.md missing", bdir)
		return
	}
	text := string(overview)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "overview.md" {
			continue
		}
		if !strings.Contains(text, strings.TrimSuffix(name, ".md")) {
			r.errf("C5", "domain %s is not referenced from %s/overview.md", name, bdir)
		}
	}
}

// C9 — plan hygiene: fix plans are deleted at completion; feature plans keep a
// Status line.
func checkPlans(root string, r *report) {
	plans := filepath.Join(agentDir(root), "plans")
	statusRe := regexp.MustCompile(`(?mi)^Status:\s*\S+`)

	_ = filepath.WalkDir(plans, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(plans, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		text := string(b)
		isFix := strings.HasPrefix(rel, "fix"+string(filepath.Separator))

		if !statusRe.MatchString(text) {
			r.errf("C9", "plans/%s has no Status: line", rel)
			return nil
		}
		if isFix && regexp.MustCompile(`(?mi)^Status:\s*complete`).MatchString(text) {
			r.errf("C9", "plans/%s is complete but still present — fix plans are deleted at completion", rel)
		}
		return nil
	})
}

// C11 — relative markdown links inside .agent/ must resolve.
func checkLinks(root string, r *report) {
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	adir := agentDir(root)

	_ = filepath.WalkDir(adir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(adir, p)
		for _, m := range linkRe.FindAllStringSubmatch(string(b), -1) {
			target := strings.TrimSpace(m[1])
			if target == "" || strings.Contains(target, "://") ||
				strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target = strings.SplitN(target, "#", 2)[0]
			if target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(p), target)); err != nil {
				r.warnf("C11", "%s links to %s, which does not exist", rel, target)
			}
		}
		return nil
	})
}

// C12 — seam declarations are well-formed (D038). Tier is data so that one
// design serves a 27B consumer and a frontier one; this checks the data.
// Absent seams: section is legitimate — a repo may not have declared them yet.
func checkSeams(m *Manifest, r *report) {
	seams, ok := m.Raw["seams"].(map[string]any)
	if !ok {
		return
	}
	known := map[string]bool{"cheap": true, "mid": true, "strong": true, "any": true}

	for _, name := range sortedKeys(seams) {
		decl, ok := seams[name].(map[string]any)
		if !ok {
			r.errf("C12", "seam %q is not a mapping", name)
			continue
		}
		tier, _ := decl["tier"].(string)
		if tier == "" {
			r.errf("C12", "seam %q declares no tier", name)
			continue
		}
		if !known[tier] {
			r.errf("C12", "seam %q declares unknown tier %q (cheap|mid|strong|any)", name, tier)
		}
		// max_task_layers is the one knob whose right answer differs by tier
		// (task granularity) — that is why it is data rather than procedure. A
		// typo here silently changes granularity, so schema-check it.
		if v, declared := decl["max_task_layers"]; declared {
			n, ok := yamlInt(v)
			if !ok || n < 1 {
				r.errf("C12", "seam %q max_task_layers must be a positive integer", name)
			}
		}
		// The auditor assignment is not a tuning knob: a false positive here
		// silently corrupts the rules->tests closure, at any target tier.
		if name == "auditor" && tier != "strong" {
			r.errf("C12", "seam %q must be strong — semantic coverage judgment is where a false pass is most costly", name)
		}
	}
}

// yamlInt reads an integer out of a YAML-decoded scalar (yaml.v3 yields int or
// int64 depending on width).
func yamlInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
