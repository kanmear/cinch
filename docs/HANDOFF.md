# P3 handoff — C6/C7 landed, fixtures green on two stacks, cinch self-harness live

Status: **Phase 3's exit items are met except wave 3.** `cinch check` is green on a clean tree in
project_deltadocs (`make check-harness` — exit 0 with two designed warnings), in both fixtures
("harness ok", zero findings), and on cinch's own repo ("harness ok"). Every mechanical audit in
`sync-docs`/`optimize-docs` prose is mapped to a checker or is judgment by design (D058). C8 is
deferred: its build ships with the P4.1 rule-ID migration, which owns the marker rollout (D057).

## Done — [P3 session 1]

**Introspection contract + C6/C7 (cinch side, D028/D056)**

- `docs/INTROSPECTION.md` defines the two producer output shapes (`types` with serialized field
  names — `json:"-"` fields by declared Go name; `routes` with uppercase method and exact path) and
  the matching rules: a type is documented by a ```go fence containing `type <Name> struct`
  (signature.md documents two structs under `## X Struct` headings — the fence is authoritative,
  not the H1), a route by an `## METHOD /path` heading with fenced lines skipped (D046).
- `checkTypes`/`checkRoutes` in `check.go`, wired into `cmdCheck`. Doc-upstream direction errors
  (documented type/field/route must exist in the producer output); coverage direction warns (real
  type/route with no doc) — `emit()` returns success on warnings, so the coverage question can't
  red the exit bar. A repo that declares no `introspect-types`/`introspect-routes` key skips the
  check, mirroring C5's missing-business-layer and C12's missing-seams semantics. Producers run
  with a 30s timeout.
- `scaffold/manifest.example.yml` gained `test: make test` — the D051-noted `{{commands.test}}`
  gap, closed because the fixtures now render `task-primitive.md` and would hit it at render time.
- Parser unit tests in `introspect_test.go` (doc-struct extraction, route-heading regex).

**Project producers + route refactor (project_deltadocs, D033 proof)**

- `backend/cmd/introspect-types` (go/ast walk of exported structs in `backend/models`, json-tag
  field names) and `backend/cmd/introspect-routes` (builds the real route table with zero-valued
  deps — handler constructors only store deps — and dumps it). Manifest keys uncommented; the
  bindings use `go run -C backend ./cmd/...` because `backend/` is its own Go module and cannot be
  entered from the repo root.
- `RegisterRoutes`'s parameter became the `RouteMux` interface (`HandleFunc(string, func(w, r))`),
  so the producer can hand registration a recording mux — `http.ServeMux` cannot enumerate its own
  patterns. Call sites unchanged (`*http.ServeMux` satisfies it).
- **First-run reconciliation (expected):** the code registered the tab segment as `{id}` in three
  signature routes and `{tabID}` in the other three; the docs consistently use `{tabID}`. The
  doc-upstream resolution: made the code match the docs — the three registrations and their three
  handlers' `PathID` reads became `tabID`. Client-visible URLs unchanged. Also fixed
  `api/users.md`'s heading `## GET /api/users/search?q=...` (query strings are not routes).
- `make check-harness` green with exactly two warnings, both the designed derivability answer:
  `GET /api/health` (liveness probe, no contract to document) and `PendingSignatureIndicator`
  (internal plumbing whose response contract lives in `api/signatures.md` prose).

**Fixtures (D017/D059)**

- `fixtures/go/` — toy Go service: `app` package with json-tagged model types, a route table
  registered through the same RouteMux pattern, both producers (`introspect-types` walks `app/`
  via go/ast with `runtime.Caller`-anchored paths; `introspect-routes` uses a recording mux).
- `fixtures/node/` — toy Node service, zero dependencies: central route table, `introspect-routes`
  dumps it; deliberately no type producer — D028's "repos may answer differently", and C6 skips
  there while C7 runs.
- Both declare the full eleven-variable template contract (`paths.business` is referenced 13×, so
  even the toys declare a business layer — overview.md plus two domain docs, exercising C5
  cross-stack), seams (C12), and test tiers with an e2e notes file. Rendered workflows and
  `index.md` are committed in the cinch repo.
- **Each fixture is its own git repo** — `repoRoot()` resolves to the fixture, not cinch
  (D042's empty-repo-marker precedent). `make fixtures` git-inits idempotently and renders + indexes
  + checks each: **"harness ok"** both.
- `.github/workflows/ci.yml` — `make test` + `make fixtures` on push/PR. Dormant until the repo is
  pushed anywhere (O005).

**Mechanical-audit closure (P3 exit item, D058)**

- Every mechanical audit mapped: doc-map regen → C1 (+ pre-commit guard); markdown links → C11;
  manifest path bindings → C4; command references in rendered workflows → C2 (generated from
  `{{vars}}`, so they cannot go stale without the render check firing). Stays prose by design:
  sync-docs' commit-message greps (detection heuristics feeding judgment) and optimize-docs'
  manifest-values-restated audit (itself a deletion mandate, D009/D013).
- `templates/optimize-docs.md` stale-references bullet rewritten: harness-era "skills" dropped,
  C2/C4/C11 named as the mechanical coverage, judgment residue left explicit. Re-rendered into
  project_deltadocs; D049 purity test green.

**Cinch self-harness (D036/D055/D060)**

- cinch's own `.agent/manifest.yml` — honest bindings for a Go CLI (self-check `./bin/cinch
  check`, `go test`, `go run .` for the shared start-* bindings, no fictional dev servers), seams,
  test tiers, `tests/` as the bound tests dir.
- `.agent/business/overview.md` — the bind is required by the shared templates; the absence of
  domain docs is the declaration that cinch has no business rules. `.agent/e2e.md` for the notes var.
- `cinch render` + `cinch index` + `cinch check` on its own repo: **"harness ok"**. `tests/` is
  real — `tests/cli_test.go` exercises the built binary (`version show`, self-check); the
  introspect parser tests stay at the module root (`package main` can't be entered from a
  subpackage). `make test` and `make fixtures` green throughout.

**Decisions logged:** D056 (C6/C7 semantics: doc-upstream errors, coverage warns, absent producers
skip; contract in `docs/INTROSPECTION.md`), D057 (C8 build ships with its P4.1 migration), D058
(mechanical-audit closure map; optimize-docs bullet rewritten), D059 (fixture shape: own git repos,
Go answers the full contract, Node routes-only), D060 (cinch renders its own harness), E006 (P3
session 1 progress event). Roadmap P3's wave-2, wave-3, and fixtures bullets annotated.

## Not done — P3 status

1. **Wave 3 (C8) is deferred, not blocked-on-anything-pending.** The rule-ID/marker rollout is P4.1
   scope; the checker build lands with it (D057). Nothing to schedule before P4.1.
2. **`O005`** (org vs personal repo) — still open; the CI workflow exists but runs nothing until the
   repo is pushed. Blocks only P0.1-adjacent concerns, i.e. nothing current.
3. **Distribution (D006/D008/D047)** — unchanged: dev clone until first release; add the
   install/refresh step to `release.sh` at first tag.
4. **Red checks: none.** project_deltadocs: 2 designed warnings (health, indicators). Fixtures and
   cinch itself: zero findings.

## Next session

Phase 3 is closed except C8-by-design. The roadmap's session-loads table names **Phase 4 (Semantic
integrity)**, which starts on the project side:

> roadmap §P4 · `domain/overview.md` · one `domain/<domain>.md` · `doc-philosophy.md` · D012
> D013 D027

P4.1 is the rule-ID rollout: author-assigned permanent IDs prefixed from `rule_prefix` front-matter
on every `domain/<domain>.md` rule, `// cinch:rule PROJ-0NN` markers above the tests that enforce
them (D027) — the migration wave 3's C8 build (which the cinch side should land in the same session
as the migration) then flips on. After P4.1, the [cinch] side of P4 is C10 diff-coupling
(warn-level), and the project side is the derivability gate (4.3) and optimize-docs' deletion
mandate (4.4). P4 planning runs through cinch's own rendered `.agent/workflows/plan-feature.md`
(D036/D060) — the primitive's first consumer.

## Notes for future template work

- **Audit decisions landed (D062/D063):** `business/` → `domain/` across both consumers; templates
  select by manifest key presence (`requires:` front-matter); fragments compose at anchors. Two
  known residual sync-docs warnings on cinch's self-check (`.agent/frontend/conventions.md`,
  `.agent/backend/troubleshooting.md`) — the routing-table fragment, deferred until variance bites.
- **Temporary pointer:** the audit's "projectX" consumer is project_deltadocs at
  `~/code/project_deltadocs` (full-stack: `api/`, `models/`, `domain/`, `plans/`). Remove this line
  when the audit doc names the real consumer.
- The `RouteMux`-interface pattern is now the demonstrated way to enumerate an `http.ServeMux`:
  registration takes an interface, the producer passes a recording mux. It shipped in both the real
  consumer and the Go fixture, so it's proven portable (D059).
- Fixture manifests must declare **all eleven template vars**, not just the ones the fixture's own
  docs use — `{{paths.business}}` alone is referenced 13× across the template set, and an
  undefined variable is a render error (D005). A business layer exists in every fixture for this
  reason, and cinch's own repo has one too (overview-only).
- `go run -C <dir>` is the module-boundary idiom when a producer lives in a nested module
  (project_deltadocs' `backend/`); the manifest command runs from the repo root.
- Fixtures inside the cinch repo need their own git repos for `repoRoot()` to resolve (D042) —
  `make fixtures` self-inits them; the outer repo tracks the working trees and never the inner
  `.git`.
- A doc heading carrying a query string (`## GET /api/users/search?q=...`) fails C7's exact-path
  match — query strings are not routes.
- C6's field parse treats the ```go fence as authoritative, not the doc's H1: a doc may document
  several structs (`signature.md` does), each in its own fence or sequentially in one fence.
