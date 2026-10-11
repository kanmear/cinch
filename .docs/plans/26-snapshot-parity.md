# 26. The index snapshot's two paths give one answer

**Tier 1 — trust the checker.** Bound by **CINCH-006** in `.docs/rules.md`. Follows
[14](14-pre-commit-checks-index.md); ships on `fix/snapshot-flagged-entries`.

## Why

`stagedSnapshot` (`internal/cinch/staged.go`) has a fast path (the working tree equals the
index, so checks read it in place) and a slow path (`git checkout-index` into a temporary
directory). They should agree, and the choice between them should cost nothing a commit
doesn't contain. On `dev` they don't:

1. **Links out of the repo.** `checkLinksInFile` resolves a target next to the doc. In a
   snapshot that's under `/tmp`, so `../../sibling/README.md` fails on the slow path and
   passes on the fast one. The slow path is wrong here: the index says nothing about files
   outside the repository.
2. **Ignored docs.** `checkLinks`, `checkIndex`, `scanRuleDocs`, `scanDocFrontmatter` and
   `orphanedGeneratedFiles` walk the filesystem. On the fast path they see a gitignored
   `.docs/local.md`; on the slow path they don't. An ignored doc with an unmarked rule blocks
   a commit only when nothing is untracked.
3. **Cost.** Any untracked file anywhere forces the slow path, so most real commits pay for a
   full `checkout-index`: 3.2 s instead of 1.8 s at 100k files (`bench/README.md`).

Two commits on the branch fixed the first two symptoms one at a time (T1, then T2 as a
stopgap). T3 fixes the cause and removes T2.

## Design

**T1. Out-of-repo link targets resolve against `repoRoot`.** `checkLinks` takes `checkRoots`.
When a resolved target falls outside `fsRoot` (and `fsRoot != repoRoot`), it is checked at the
same relative position from `repoRoot`. Targets inside the repo keep resolving in the snapshot.
Working-tree callers (`buildDocLinks`, tests) pass `workingTreeRoots`.

**T2. Ignored files in the docs or hooks directory rule out the fast path.** A stopgap, shipped
in an earlier commit on the branch and removed by T3: after the working tree matched the index,
`git ls-files --others --ignored` over the docs and hooks paths sent any ignored file to the
snapshot. It cost a git call per commit and covered only those two directories.

**T3. Checks read only what the commit contains, and untracked files stop forcing a
snapshot.**

- `checkRoots` gains `index *indexSet`, nil in working-tree mode. `readIndex` builds it once
  from the listing it already has (the repo-relative files plus every ancestor directory), so
  it is immutable before any check runs. Both paths set it; on the snapshot path it agrees with
  what `checkout-index` wrote.
- `func (roots checkRoots) inCommit(path string) bool` takes an `fsRoot`-joined path. True
  when `index` is nil, when the path is `fsRoot` itself, or when it is outside `fsRoot` (T1's
  domain). Otherwise true only for an indexed file or an ancestor directory of one.
- Every filesystem read that a staged check does goes through it. The four doc walks share
  `checkRoots.walkDocs` and `collectDocs` (over a new `mdscan.WalkMarkdownFilesIf`, which
  doesn't descend into a rejected directory); `scanRuleDocs`, `scanDocFrontmatter` and
  `checkIndex` take a `checkRoots`, and working-tree callers pass `workingTreeRoots`. Link
  targets inside the repository must also be `inCommit`. `checkGenerated` reads destinations
  through `checkRoots.readFile` and treats one that isn't `inCommit` as absent, in both the
  "rendered at all?" probe and the per-file comparison; `orphanedGeneratedFiles` skips entries
  that aren't. The marker scan already uses `indexFiles`.
- A `docsRoot` the commit lacks entirely (a new, untracked `.docs`) fails the walk with the
  same "lstat ...: no such file or directory" a snapshot gives, so the two paths print the
  same finding.
- The manifest is read by path, not walked. If `cinch.yml` exists on disk but isn't indexed,
  take the snapshot path.
- The fast path then needs only: no assume-unchanged or skip-worktree entry, `git diff
  --quiet`, and the manifest guard. `ls-files --others` and T2's probe are gone;
  `workingTreeMatchesIndex` became `trackedFilesMatchIndex`, and the snapshot half of
  `stagedSnapshot` became `checkoutSnapshot` so a test can force it. The doc comment on
  `stagedSnapshot` no longer describes an asymmetry.
- Accepted limit: a tracked symlink whose target is untracked reads that target on the fast
  path. Nothing reads through symlinks deliberately, and no consumer is known to rely on one.

## Tests (`internal/cinch/staged_test.go`, each marked `cinch:rule CINCH-006`)

- T1: an out-of-repo link to an existing file passes on the snapshot path; one to a missing
  file fails on both paths.
- T2: an ignored doc with an unmarked rule doesn't block, with a clean tree. After T3 it holds
  on both paths, and the test checks both.
- T3: an untracked file alone keeps the fast path (`fsRoot == repoRoot`); on the fast path an
  untracked doc with an unmarked rule doesn't block, a link to an untracked file fails, and an
  untracked generated-looking file in the workflows directory isn't an orphan; a rendered
  file the commit drops is reported missing; rendered output that is only on disk reads as not
  rendered; an untracked `cinch.yml` forces the snapshot path; fast and slow paths print
  identical output for the same untracked and ignored files, and for a docs directory the
  commit lacks. `TestCheckRootsInCommit` covers `fsRoot` spelled `.`, absolute, relative and
  `../`. Tests that used `untracked.txt` to force the slow path switch to an unstaged edit of
  a tracked file.

## Status

Shipped in `a937164` (`fix/snapshot-flagged-entries`). Kept for provenance.
