# cinch — surface hardcoded-docs-path candidates instead of a hand list

Status: **proposed** — not started.

## Context

From the "Checking the Checker" audit (2026-08-18), IDEA-02.

`project_deltadocs/TODO.md` § "Docs-root rename readiness" maintains, entirely
by hand, a checklist of the ~25 places across the repo that hardcode the
literal `.agent` path — swept manually on 2026-08-17 via a one-off `grep -rn
'\.agent'` the maintainer ran and pasted in. This is exactly the shape of
artifact cinch's own philosophy warns against (a hand-authored index that can
only ever be as fresh as the last time someone remembered to re-run the grep)
— and it exists specifically because `cinch move-docs` is honest that it only
handles "the load-bearing half of a rename": the directory, `paths.docs`
itself, and the re-render. Everything else that spells the docs root as a
literal string is out of scope for that command by design (README's own
description of `move-docs`).

## Design

Not auto-fixing prose — cinch's direction rule and principle 5 both argue
against cinch mutating arbitrary project text it doesn't own the shape of.
Surfacing candidates is different: a read-only scan, the same kind of thing
`cinch index` and `cinch workflows` already do (compute-on-demand, nothing
persisted).

- A new subcommand, e.g. `cinch move-docs --scan` (prints candidates without
  moving anything) or a standalone `cinch paths` — decide the shape during
  implementation; `--scan` keeps the affordance next to the command that
  already owns "the docs path matters," a standalone command keeps
  `move-docs`'s existing contract (it moves, full stop) undiluted. Lean
  standalone unless implementation reveals real code-sharing pressure with
  `docsmove.go`.
- Walks the repo (same skip-list as `scanRuleMarkers` — `.git`, build output,
  the docs dir itself is expected to reference itself and isn't a finding)
  for occurrences of the literal current `paths.docs` value as a substring,
  outside of markdown link syntax `checkLinks` already resolves correctly
  (those aren't hardcoding a problem — they're the working case) and outside
  the manifest's own `paths.docs:` line (also expected, not a finding).
- Output is a flat list: file, line, matched text — deliberately *not* a
  check (no exit-code failure, no `cinch check` integration). A repo is
  allowed to reference its docs path in prose; this is a discovery aid for
  the specific, rare moment someone is about to run `move-docs`, not a
  standing rule anything should be red over. Same posture as `cinch ignores`
  and `cinch context`: "always exits 0... reports state, never a verdict."

## Steps

1. Confirm the shape decision (flag on `move-docs` vs. standalone command)
   against `docsmove.go`'s current structure once implementation starts —
   this plan intentionally leaves it open rather than guessing ahead of
   reading the code closely.
2. Implement the scan: reuse the walk-and-skip pattern from
   `scanRuleMarkers` (`internal/cinch/rules.go`) rather than writing a new
   one from scratch.
3. Exclude false-positive shapes: relative markdown links under the docs
   root (already covered, working correctly, by `checkLinks`), the
   manifest's own `paths.docs` value line, anything inside `.git`.
4. Print results in a stable, greppable format — file:line: matched text —
   so it composes with normal shell tools the way `cinch ignores`' output
   does.
5. Document it in `README.md` near the existing `cinch move-docs`
   description, and cross-reference from the `move-docs` bullet ("see also
   the `--scan`/`paths` command for what it doesn't move for you").

## Verification

- Run against `project_deltadocs` (with permission — this only reads, never
  writes) and diff the output against its hand-maintained `TODO.md` checklist
  as of the 2026-08-17 sweep: the scan should surface a closely overlapping
  set (allowing for drift since that date) without requiring a human to
  re-run `grep` by hand.
- Confirm it's silent (or near-silent, modulo real prose mentions) against
  cinch's own repo, which references `.docs` in exactly the places it's
  supposed to.
- `go vet ./...`, `make test`.

### Critical files

- `internal/cinch/rules.go` — `scanRuleMarkers`'s walk-and-skip pattern to
  reuse.
- `internal/cinch/docsmove.go` — the command this either extends or sits
  beside.
- `internal/cinch/links.go` — reference for how the existing link-resolution
  check tells a legitimate docs-path reference apart from a broken one; the
  new scan needs an analogous "this is fine" carve-out for working links.
- `README.md` — `move-docs` documentation.

### Relationship to other plans

Independent of the others. Lower priority than the rule-ID and coupling-window
plans per the audit's own verdict — a convenience tool for an infrequent
operation (a docs-root rename), not a gap in cinch's core enforcement loop.

New capability: on completion, mark status `complete` and keep the file.
