// Package check implements the harness checkers (C1–C12) and `cinch check`
// itself: doc-upstream and lateral validation of the corpus (D009).
package check

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cinch/internal/index"
	"cinch/internal/manifest"
	"cinch/internal/render"
)

// Wave one of the checker layer (D030): C1, C2, C3, C4, C5, C9, C11, C12.
// Wave two: C6/C7 (introspection, D028). C8 (rule markers, D027) landed with
// the P4.1 migration (D057/D064) — see rule.go. P4.2: C10 (diff-coupling,
// D065) — see diff.go.
//
// D009 — the direction rule: only doc-upstream and lateral checks belong here.
// A check that validates a doc's copy of a machine-readable fact is
// doc-downstream; delete the duplication instead of adding the check.

// Run implements `cinch check`.
func Run(root string) error {
	r := &report{}

	m, err := manifest.LoadManifest(root)
	if err != nil {
		r.errf("C3", "%v", err)
		return emit(r)
	}

	checkIndex(root, r)            // C1
	checkRendered(root, m, r)      // C2
	checkCommandIDs(m, r)          // C3
	checkManifestPaths(root, m, r) // C4
	checkDomains(root, m, r)       // C5
	checkTypes(root, m, r)         // C6
	checkRoutes(root, m, r)        // C7
	checkRules(root, m, r)         // C8
	checkDiffCoupling(root, m, r)  // C10
	checkPlans(root, r)            // C9
	checkLinks(root, r)            // C11
	checkSeams(m, r)               // C12

	return emit(r)
}

// C1 — the committed index must match a fresh generation.
func checkIndex(root string, r *report) {
	want, err := index.BuildIndex(root)
	if err != nil {
		r.errf("C1", "building index: %v", err)
		return
	}
	got, err := os.ReadFile(index.IndexPath(root))
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
// cinch:rule HARNESS-004 — rendered output is never hand-edited (C2 tamper branch)
func checkRendered(root string, m *manifest.Manifest, r *report) {
	files, err := render.RenderAll(root, m)
	if err != nil {
		r.errf("C2", "%v", err)
		return
	}
	for _, f := range files {
		dst := filepath.Join(render.WorkflowsDir(root), f.Rel)
		raw, err := os.ReadFile(dst)
		if err != nil {
			r.errf("C2", "workflows/%s not rendered — run `cinch render`", f.Rel)
			continue
		}
		hdr, body := render.SplitGenerated(string(raw))
		if hdr == "" {
			r.errf("C2", "workflows/%s has no generated header — is it really generated?", f.Rel)
			continue
		}
		switch {
		case render.HeaderHash(hdr) != render.BodyHash(body):
			r.errf("C2", "workflows/%s was hand-edited — revert it and edit templates/%s", f.Rel, f.Rel)
		case body != f.Body:
			r.errf("C2", "workflows/%s is stale against the manifest — run `cinch render`", f.Rel)
		}
	}
}

// C3 — every test tier's cmd id resolves under development.commands.
func checkCommandIDs(m *manifest.Manifest, r *report) {
	cmds := m.CommandKeys()
	tiers := m.TestTierCommands()
	for _, tier := range manifest.SortedKeys(tiers) {
		cmd := tiers[tier]
		if !cmds[cmd] {
			r.errf("C3", "test tier %q declares cmd %q, which has no key under development.commands", tier, cmd)
		}
	}
}

// C4 — every path binding points at something that exists.
func checkManifestPaths(root string, m *manifest.Manifest, r *report) {
	pv := m.PathValues()
	for _, k := range manifest.SortedKeys(pv) {
		p := filepath.Join(root, pv[k])
		if _, err := os.Stat(p); err != nil {
			r.errf("C4", "manifest %s -> %s does not exist", k, pv[k])
		}
	}
}

// C5 — the domain list is the filesystem; overview.md must agree with it.
func checkDomains(root string, m *manifest.Manifest, r *report) {
	bdir := m.Vars["paths.domain"]
	if bdir == "" {
		// No domain layer declared. Legitimate — a consumer with no domain
		// rules declares absence as shape (D063), and the templates that
		// require the binding are not rendered for it.
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
	plans := filepath.Join(manifest.AgentDir(root), "plans")
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

// C11 — relative markdown links and backticked path tokens inside .agent/
// must resolve. Links are doc-relative markdown cross-references; a
// backticked token that looks like a path is a reference in prose — the
// audit's §1.2 findings were all backtick-quoted paths, not links, so a
// link-only checker missed every one of them. `.agent/...` tokens are
// root-relative (D045), everything else resolves from the repo root.
func checkLinks(root string, r *report) {
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	tickRe := regexp.MustCompile("`([^`]+)`")
	adir := manifest.AgentDir(root)

	// resolve reports whether target resolves; pathless and non-relative
	// targets (URLs, anchors, mailto) are always fine.
	resolve := func(p, target string) bool {
		target = strings.TrimSpace(target)
		if target == "" || strings.Contains(target, "://") ||
			strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			return true
		}
		target = strings.SplitN(target, "#", 2)[0]
		if target == "" {
			return true
		}
		_, err := os.Stat(filepath.Join(filepath.Dir(p), target))
		return err == nil
	}

	// tickPath decides whether a backticked token is a path reference rather
	// than a plain word: it must contain a slash, no whitespace, and no
	// placeholder (`<...>`, brackets, an ellipsis). A trailing slash is a
	// directory reference — a convention location that comes into existence
	// with its first file — and "N/A"-shaped abbreviations are not paths.
	tickPath := func(tok string) bool {
		if !strings.Contains(tok, "/") || strings.ContainsAny(tok, "<>[]{}* \t") ||
			strings.Contains(tok, "...") || strings.HasSuffix(tok, "/") {
			return false
		}
		for _, r := range tok {
			if r != '/' && (r < 'A' || r > 'Z') {
				return true
			}
		}
		return false // all-caps abbreviation like `N/A`
	}

	// resolveTick resolves a backticked path from the repo root, or from the
	// document's own directory (index.md uses `workflows/...` since it sits
	// in .agent/; templates' `.agent/...` tokens are root-relative, D045).
	resolveTick := func(p, tok string) bool {
		for _, base := range []string{root, filepath.Dir(p)} {
			clean := strings.TrimPrefix(tok, ".agent/")
			if clean != tok {
				base = filepath.Join(root, ".agent")
			}
			if _, err := os.Stat(filepath.Join(base, clean)); err == nil {
				return true
			}
		}
		return false
	}

	_ = filepath.WalkDir(adir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(adir, p)
		// Generated files carry the cinch header; their whole body is
		// template-authored prose whose tokens are expected to resolve.
		// Hand-authored docs may reference the project's own source tree,
		// imports, and aliases — not checkable from here — so their
		// backticked tokens are only checked when explicitly `.agent/`-
		// prefixed, the one root-relative convention this repo defines
		// (D045).
		generated := strings.HasPrefix(string(b), render.HeaderPrefix)
		// References inside fenced code blocks are illustrative examples, not
		// cross-references (e.g. a doc template showing [<Related> Rules](<related>.md)).
		// Checking them would flag every placeholder, so skip fenced lines.
		inFence := false
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
				if !resolve(p, m[1]) {
					r.warnf("C11", "%s links to %s, which does not exist", rel, strings.TrimSpace(m[1]))
				}
			}
			for _, m := range tickRe.FindAllStringSubmatch(line, -1) {
				if !tickPath(m[1]) {
					continue
				}
				if !generated && !strings.HasPrefix(m[1], ".agent/") {
					continue
				}
				if !resolveTick(p, m[1]) {
					r.warnf("C11", "%s refers to `%s`, which does not exist", rel, m[1])
				}
			}
		}
		return nil
	})
}

// C12 — seam declarations are well-formed (D038; allow lists per D067). Tier
// is data so that one design serves a 27B consumer and a frontier one; this
// checks the data. Absent seams: section is legitimate — a repo may not have
// declared them yet.
// cinch:rule HARNESS-002 — the auditor seam is never cheap (C12)
func checkSeams(m *manifest.Manifest, r *report) {
	seams, ok := m.Raw["seams"].(map[string]any)
	if !ok {
		return
	}
	known := map[string]bool{"cheap": true, "mid": true, "strong": true, "any": true}

	for _, name := range manifest.SortedKeys(seams) {
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
		// allow: names the development.commands ids a seam's workflows may run
		// (D067). Data for per-harness enforcement glue — a declaration gates
		// nothing by itself — but an id that does not resolve is a typo that
		// would silently widen or narrow a seam, so resolve it like C3.
		if v, declared := decl["allow"]; declared {
			ids, ok := v.([]any)
			if !ok || len(ids) == 0 {
				r.errf("C12", "seam %q allow must be a non-empty list of command ids", name)
			} else {
				cmds := m.CommandKeys()
				for _, id := range ids {
					s, ok := id.(string)
					if !ok {
						r.errf("C12", "seam %q allow entries must be strings", name)
						continue
					}
					if !cmds[s] {
						r.errf("C12", "seam %q allow %q is not a declared development.commands id", name, s)
					}
				}
			}
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

// C6 — models/*.md must describe real types (doc-upstream), and real types
// should be documented (coverage, warn). The introspection producer is a
// manifest-declared command (D028); the JSON contract it must emit is
// docs/INTROSPECTION.md. Absent a declared producer or a models/ dir, the
// check skips — same absence-is-a-declaration semantics as C5 and C12.
func checkTypes(root string, m *manifest.Manifest, r *report) {
	cmd := m.Vars["commands.introspect-types"]
	if cmd == "" {
		return
	}
	mdir := filepath.Join(manifest.AgentDir(root), "models")
	if _, err := os.Stat(mdir); err != nil {
		return
	}

	out, err := runIntrospection(root, cmd)
	if err != nil {
		r.errf("C6", "introspect-types: %v", err)
		return
	}
	var spec struct {
		Types []struct {
			Name   string   `json:"name"`
			Fields []string `json:"fields"`
		} `json:"types"`
	}
	if err := json.Unmarshal(out, &spec); err != nil {
		r.errf("C6", "introspect-types: output is not valid JSON — see docs/INTROSPECTION.md (%v)", err)
		return
	}

	real := map[string]map[string]bool{}
	for _, t := range spec.Types {
		fields := map[string]bool{}
		for _, f := range t.Fields {
			fields[f] = true
		}
		real[t.Name] = fields
	}

	documented := map[string]bool{}
	_ = filepath.WalkDir(mdir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(mdir, p)
		for _, dt := range docStructs(p) {
			documented[dt.name] = true
			fields, ok := real[dt.name]
			if !ok {
				r.errf("C6", "models/%s documents type %q, which introspect-types does not report", rel, dt.name)
				continue
			}
			for _, f := range dt.fields {
				if !fields[f] {
					r.errf("C6", "models/%s documents field %q on %q, which introspect-types does not report", rel, f, dt.name)
				}
			}
		}
		return nil
	})

	for name := range real {
		if !documented[name] {
			r.warnf("C6", "type %q is not documented in any models/*.md", name)
		}
	}
}

// C7 — api/*.md routes must be registered (doc-upstream), and registered
// routes should be documented (coverage, warn). A route is documented by an
// `## METHOD /path` heading; headings inside fenced code blocks are skipped
// (D046), same as C11.
func checkRoutes(root string, m *manifest.Manifest, r *report) {
	cmd := m.Vars["commands.introspect-routes"]
	if cmd == "" {
		return
	}
	adir := filepath.Join(manifest.AgentDir(root), "api")
	if _, err := os.Stat(adir); err != nil {
		return
	}

	out, err := runIntrospection(root, cmd)
	if err != nil {
		r.errf("C7", "introspect-routes: %v", err)
		return
	}
	var spec struct {
		Routes []struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(out, &spec); err != nil {
		r.errf("C7", "introspect-routes: output is not valid JSON — see docs/INTROSPECTION.md (%v)", err)
		return
	}

	real := map[string]bool{}
	for _, rt := range spec.Routes {
		real[rt.Method+" "+rt.Path] = true
	}

	documented := map[string]bool{}
	_ = filepath.WalkDir(adir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(adir, p)
		inFence := false
		for _, line := range strings.Split(readFile(p), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, h := range routeHeadingRe.FindAllStringSubmatch(line, -1) {
				key := h[1] + " " + h[2]
				documented[key] = true
				if !real[key] {
					r.errf("C7", "api/%s documents %s %s, which is not a registered route", rel, h[1], h[2])
				}
			}
		}
		return nil
	})

	for key := range real {
		if !documented[key] {
			r.warnf("C7", "route %s is not documented in any api/*.md", key)
		}
	}
}

// runIntrospection shells out to a manifest-declared introspection command
// (D024: checkers are compiled in; the introspection escape hatch is the
// declared-command contract). A producer that hangs should fail, not hang the
// check.
func runIntrospection(root, cmdline string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdline)
	cmd.Dir = root
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(errb.String())
		if tail != "" {
			return nil, fmt.Errorf("%v — %s", err, tail)
		}
		return nil, err
	}
	return out.Bytes(), nil
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}
