# cinch — document a workflow tool-permission convention (data, not code)

Status: **complete**.

## Context

From the "Checking the Checker" audit (2026-08-18), GAP-04 + IDEA-01.

`cinch-v0/docs/ROADMAP.md` Phase 5 — a Claude Code PreToolUse guard and an
opencode auditor envelope, both manifest-driven, `allow:` command-id lists
declared in both harness glues, verified end-to-end in two harnesses — was the
most validated piece of engineering the pre-purge system had. Nothing in the
purge's own evidence trail (the pi-audit confabulation record,
`records/pi-audits/README.md`) implicates the permission layer in the failure
that triggered the 2026-08-07 purge; that failure was specifically the
semantic auditor's judgment on rule coverage. The permission seam went out
anyway, because the purge was total rather than surgical (PRINCIPLES.md's own
instruction: read PRINCIPLES.md "and nothing else about the old design").

The audit's proposed shape is deliberately the narrowest useful slice, not a
resurrection of v0's Phase 5 apparatus: a documented convention for declaring
per-workflow tool-call allow-lists, consumed by a harness's own permission
system — cinch never executes or enforces it. This matters because of what
`README.md`'s "A richer project manifest" section (lines 395-425) already
establishes: `cinch.yml` only binds what cinch itself reads (`paths.*`,
`hooks.*`, `commit.pattern`, `require.cinch`); anything else belongs in the
project's own `manifest.yml`, which cinch never reads or validates by design
(principle 5). A permission allow-list is exactly that kind of value —
project- and harness-specific, not something cinch's core has any business
parsing.

## Design

This is closer to a documentation addition than a feature: recommend a key
shape for `manifest.yml`, don't add anything cinch itself reads.

- Extend `manifest.example.yml` (the starting point `cinch init` writes) with
  a commented-out example block, e.g.:

  ```yaml
  # Optional: per-workflow tool-call allow-lists, consumed by your harness's
  # own permission system (cinch never reads or enforces this).
  # permissions:
  #   docs-audit-coverage: [Read, Grep]
  #   dev-execute-plan: [Read, Edit, Bash]
  ```

- Add a paragraph to README's "A richer project manifest" section describing
  the convention and explicitly naming what it resurrects from v0 (a link to
  this plan file, or a short inline note, for anyone who finds the old
  ROADMAP.md and wonders why Phase 5 vanished) and what it deliberately
  doesn't: cinch has no PreToolUse guard, no `cinch hook`-driven enforcement
  of this key, no opinion on which harnesses can consume it.
- No code change to `internal/cinch/manifest.go` or any checker — the whole
  point is that this stays outside what cinch parses, matching how the richer
  `manifest.yml` already works for `taxonomy.test_tiers` and everything else
  in that file.

## Steps

1. Draft the `manifest.example.yml` addition and the README paragraph
   together, so the two stay consistent (example syntax must match what the
   prose describes).
2. Decide the key name (`permissions`, `workflow-permissions`,
   `tool-permissions` — check for collision with anything already documented
   in `manifest.example.yml` before picking).
3. Cross-check against `project_deltadocs/.claude/agents/auditor.md` and any
   existing permission-adjacent convention already in that repo's
   `manifest.yml`, so the documented shape isn't inventing a second
   incompatible convention where a first one already exists informally.

## Verification

- No automated verification beyond `make test` staying green (no code
  touched).
- Manual: confirm the README addition reads as clearly optional and
  clearly out of cinch's enforcement scope — a reader should not come away
  thinking `cinch check` will ever fail on this key.

### Critical files

- `internal/cinch/init.go` / wherever `manifest.example.yml`'s content is
  generated or templated from — confirm the exact source before editing.
- `README.md` § "A richer project manifest" (lines 395-425).
- Read-only reference: `cinch-v0/docs/ROADMAP.md` § Phase 5,
  `project_deltadocs/.claude/agents/auditor.md`.

### Relationship to other plans

Independent of all others. Smallest-scope item of the six — worth doing
precisely because it's cheap and directly answers a real, cited gap (GAP-04)
without reopening any of the surface area the purge correctly closed.

Documentation/convention addition: on completion, mark status `complete` and
keep the file as the record of why this is data-only and what it deliberately
does not resurrect.
