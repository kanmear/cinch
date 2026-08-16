# cinch — absolute `paths.docs` silently splits read from write

Status: **proposed** — not started.

## Context

README (`README.md:82`) says `paths.docs` accepts a value "relative to the
project root, or an absolute path." The absolute case does not work, and the
way it fails is worse than a wrong doc line: **all five checks go green while
the rendered corpus is invisible to every check that reads it.**

Three code sites, two different resolution rules:

- `ResolveDocsRoot` (`internal/cinch/check.go:31-41`) **honors** an absolute
  value — it explicitly returns it as-is. So `links`, `rules`, `coupling`,
  plus `ignores`, `index`, and `workflows`, all read from the absolute path.
- `renderAll` (`internal/cinch/render.go:115`) takes the raw value from
  `docsPathValue` and uses it as a *repo-relative* `Dest` string; `CmdRender`
  (`render.go:212`) then does `filepath.Join(root, f.Dest)`. `filepath.Join`
  does not special-case an absolute second argument, so render **writes** to
  `<repo-root>/<abs-path>/…`.
- `checkGenerated` (`internal/cinch/generated.go:61`) repeats the same join,
  so it verifies exactly the nested copies render produced.

With `paths.docs = /abs/path`, the result is:

| | resolves to |
|---|---|
| `render` writes | `<repo-root>/abs/path/workflows/…` |
| `generated` verifies | `<repo-root>/abs/path/workflows/…` — clean |
| `links`/`rules`/`coupling` scan | `/abs/path` — empty, so clean |

Every check passes. Worse, `{{paths.docs}}` is substituted into the rendered
workflows as the absolute value (`render.go:127`), so the workflows instruct
an agent to write docs into `/abs/path` — a location `render` never populates
but the checks *do* scan. The one guarantee the single-root design exists to
provide ("a rule doc a workflow tells an agent to write is guaranteed to be
one `cinch check` actually scans", `README.md:83-85`) is exactly what this
configuration breaks.

The drift-checker has a configuration in which drift is undetectable. That
is the reason to close this rather than leave it as a documentation nit.

## Decision — settled: the docs root lives inside the repo

Two options were open: **A**, support absolute paths properly (teach
`renderAll` to emit an absolute destination and `checkGenerated` to resolve
the same way — at the cost of splitting `renderFile.Dest` into a write path
and a display path, since it currently doubles as the repo-relative string in
`Finding.File` and `output.Step` messages); or **B**, refuse a docs root that
isn't inside the repo.

**Decided: B, in its stronger form** (author, 2026-08-16). `paths.docs` must
resolve to a location **inside the repository** — which rejects not only
absolute paths but relative ones that escape via `..`. The same rule applies
to **`paths.hooks`**, which has the identical defect through the identical
`filepath.Join` (`render.go:117`, `render.go:212`) and no reason to differ.

Why this rather than A: no known consumer wants an out-of-tree root — this
repo uses `.docs`, `project_deltadocs` uses `.agent` — so A would add a
second path representation to hold in service of zero users, against the
holdability gate. A refusal at load time is decidable, cannot rot, and turns
an undetectable-drift configuration into an immediate, exactly-actionable
error.

This is the *narrowing* choice: it removes a capability the README claims
rather than delivering it. Revisit only if a real repo needs an out-of-tree
root, at which point A is the right answer and the analysis above is the
starting point.

### The invariant already exists, as a silent degradation

`checkCoupling` (`coupling.go:39-50`) already computes exactly this
condition — `filepath.Rel` from the repo root, testing for `..` — and on
violation returns `NoOp: "paths.docs is outside the repository"`. So the
constraint is not new; today it is enforced by one check going dark while the
other four carry on, which is the weakest possible form of it.

Making it a load-time refusal means **that branch becomes unreachable and
should be deleted**. This fix nets out as a deletion plus one validation, not
as added machinery — worth stating, because a plan that adds a rule usually
adds code, and this one doesn't.

## Step 1 — close the split

Validate at manifest load, so every command fails identically rather than
each one resolving in its own way. Placement matters: `parseManifestFile` is
deliberately generic — it flattens YAML into a `dotted.key -> value` bag and
knows nothing about specific keys, per the design note at `manifest.go:20-24`
("nesting in the file is notation… it doesn't grow the manifest a schema").
Key-aware validation must not go there. Put it in a named validation step
called by **both** `loadManifest` and `loadManifestOptional`
(`manifest.go:37,51`), which are the two entry points and both already return
errors.

`filepath.IsLocal` (Go 1.20+; the module is `go 1.22`) is the exact
primitive: it reports whether a path is non-empty, not absolute, and does not
escape the directory it is relative to. Prefer it over a hand-rolled
`IsAbs` + `..` scan.

The error must name the key, the offending value, and the constraint.

Then remove what the guarantee makes dead:

- `ResolveDocsRoot`'s `filepath.IsAbs` branch (`check.go:37-39`) and its
  doc-comment sentence "An absolute value is used as-is" — leaving that in
  place is the same class of false claim this plan exists to remove.
- `checkCoupling`'s outside-the-repo branch (`coupling.go:47-49`). The
  `filepath.Abs` resolution above it stays; only the escape test and its
  `NoOp` go.

## Step 2 — mutation fixture

Per the repo's own rule, the fix ships with fixtures proving the failing
state, asserted against the message rather than just a non-nil error:

- absolute `paths.docs` → load error.
- escaping relative `paths.docs` (`../elsewhere`) → load error. This is the
  case the "must be relative" framing would have missed.
- absolute or escaping `paths.hooks` → load error.
- ordinary relative values (`.docs`, `.agent`, `docs/`) → unaffected.

Hand-verify each genuinely fails: revert the validation, confirm red.

## Step 3 — README

Delete the "or an absolute path" clause at `README.md:82` and replace it with
the constraint, since a silently-narrowed capability is its own small drift.

## Verification

- `make test` green, including the new fixtures.
- The currently-green-but-wrong configuration must no longer report five
  clean checks: in a scratch repo, set `paths.docs` to an absolute path, run
  `cinch render` then `cinch check`. Today: renders nested, reports clean.
  After: an error at load, naming the key.
- `./bin/cinch check` in this repo still exits 0 (relative `.docs` path
  unaffected), and `project_deltadocs` (relative `.agent`) is likewise
  untouched — confirm rather than assume.
- `checkCoupling` still behaves identically on every *reachable* input: the
  branch removed was unreachable only *after* the validation lands, so run
  the coupling fixtures specifically, not just the suite total.

### Critical files

- `internal/cinch/manifest.go` — the load-time validation, called from
  `loadManifest` and `loadManifestOptional`, not from `parseManifestFile`
- `internal/cinch/check.go` — `ResolveDocsRoot`'s `IsAbs` branch and its doc
  comment
- `internal/cinch/coupling.go:47-49` — the now-dead outside-repo branch
- `README.md:82`

### Relationship to other plans

Independent, and small enough to land in one sitting. `cinch-context.md`
touches `ResolveDocsRoot` as a reader; if both are in flight, land this one
first so context is written against the settled resolution rule.
Unrelated to `docs-path-migration.md`, which is about *changing* the value
safely, not about what values are legal.
