# 14. Pre-commit checks what's staged, not the working tree

**Status: shipped in `1af2986` (`fix/pre-commit-checks-index`). Kept for provenance.** Bound
by **CINCH-006** in `.docs/rules.md`. Depended on 13 (both touch `check.go`).

## Why

The core guarantee was broken in both directions, reproduced with a 0.2.x build:

1. **Wrongly blocked.** An unmarked rule left unstaged in a rule doc blocked an unrelated
   staged commit (`rules error … AUTH-005: no // cinch:rule marker`). The only way out was
   `--no-verify`, which disables everything — the habit an agent in the pi+Qwen field
   report learned.
2. **Wrongly allowed.** A staged unmarked rule passed when its marker existed only as an
   unstaged edit, or only in an untracked file (`gitutil.ScannableFiles` includes
   `--others`). HEAD was then red.

Cause: `cmdHookPreCommit` collected staged paths but used them only for `dispatchHooks`
and `buildImpact`; every check (`checkLinks`, `checkIndex`, `scanRuleDocs`,
`markerScanFiles`, `checkGenerated`) read the filesystem.

## Design: check a snapshot of the index

1. **`checkRoots`** separates where git runs from where content is read:
   `repoRoot` (git commands, `rules.roots` siblings, `core.hooksPath`), `fsRoot` (equal to
   `repoRoot`, or an index snapshot), `indexFiles` (staged mode only). Plain `cinch check`
   keeps `fsRoot == repoRoot` and nil `indexFiles`.
2. **`stagedSnapshot`** (`internal/cinch/staged.go`). Fast path: no unstaged tracked
   changes and no untracked files means the working tree equals the index, so
   `fsRoot = repoRoot`. Otherwise `git checkout-index --all --force --prefix=<tmp>/`, with
   a cleanup that is always safe to defer. Git never runs with its working directory inside
   the snapshot, so `GIT_DIR`/`GIT_INDEX_FILE` (set by `git commit -a` and
   `git commit <paths>`) are honored.
3. Content checks, the manifest and `ResolveDocsRoot` read `fsRoot`.
4. The marker scan takes `indexFiles` for the primary root (untracked files drop out;
   gitlinks and missing entries are skipped). `rules.roots` siblings still resolve against
   `repoRoot` and are scanned as they are.
5. `checkGenerated` reads files under `fsRoot` but runs `git show HEAD:cinch.yml` in
   `repoRoot`.
6. Findings are relabeled so output never names the temporary directory.
7. Wiring: the hook uses the staged variant; `commitMsgChecks` is unchanged; `CmdUpgrade`
   stays on the working tree, since it has just written unstaged rendered files.
8. `cinch check --staged` runs the hook's path by hand; combining it with `--changed` is a
   usage error.

As shipped, `buildImpact` and `dispatchHooks` kept reading the working tree. The impact
advisory moved onto the snapshot later (`fix/impact-reads-snapshot`); project hook scripts
still see the working tree.

## Tests (`internal/cinch/staged_test.go`)

Unstaged rule edit doesn't block; staged unmarked rule hidden by an unstaged marker blocks;
a marker only in an untracked file doesn't count; `GIT_INDEX_FILE` honored; finding paths
are repo-relative; snapshot is cleaned up; sibling roots resolve against the repo;
`generated` reads the index; the fast path matches the slow one; upgrade still checks the
working tree.
