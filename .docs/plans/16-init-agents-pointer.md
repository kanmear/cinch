# 16. `cinch init` writes the AGENTS.md pointer

**Status: shipped in `5121e4e` (`feature/init-agents-pointer`). Kept for provenance.**
Depended on nothing.

## Why

The layer ablation (48 runs) found that a docs corpus with no pointer from `AGENTS.md`
scored exactly like having no docs at all — the most robust measured finding — yet
`cinch init` no longer wrote one. A fresh init gave a green check and a corpus agents never
found.

## Change

1. `ensureAgentPointer(repoRoot, docsPath)`:
   - `AGENTS.md` missing: write `# Agent instructions` followed by the block below.
   - Exists and already mentions `cinch` (case-insensitive): left untouched, with a step
     line saying so.
   - Exists without it: append the block; never rewrite existing content.
   - `agents.pointer: false` in `cinch.yml`: do nothing, print a skip.
2. The block names `<paths.docs>/`, `cinch index`, `cinch workflows`/`cinch workflow
   <name>`, the bold-ID rule convention and its `cinch:rule <ID>` marker, the "update the
   rule and its test together" norm, and that `cinch check` must pass before committing.
   This repo's own `AGENTS.md` is the rendered example.
3. Called from `CmdInit` after render and hook activation, git or not; no prompt, since it
   only adds content and is idempotent.
4. If `CLAUDE.md` exists and references neither `AGENTS.md` nor cinch, init prints a hint
   suggesting `@AGENTS.md`. It never edits `CLAUDE.md`.

## The trap

`AGENTS.md` sits outside the docs root, so the marker scan reads it. The block must not
contain a marker-shaped string — a comment leader followed by `cinch:rule` and a real ID
would create a phantom marker. The block keeps `<ID>` as a placeholder and the example ID
in prose with no comment leader. A test guards this.

## Tests (`init_test.go`)

Missing file created with a custom docs path; existing content preserved as a prefix;
cinch-mentioning file byte-identical; idempotent across two runs; opt-out creates nothing;
no phantom markers or rule items in the block; end to end, `rules` skips after init; the
`CLAUDE.md` hint appears only when the reference is missing.
