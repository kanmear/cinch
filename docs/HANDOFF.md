# P4.2 handoff — C10 diff-coupling landed, D065 direction decision logged

Status: **Phase 4.2 exit items are met.** C10 is live on both consumers: a change touching a
domain's `owns:` paths without its domain doc warns at the working-tree boundary, silent on clean
trees, skip-when-absent by design. Remaining P4 work is the project side — 4.3 (derivability
gate) and 4.4 (optimize-docs deletion mandate) — plus O005. See Next session.

## Done — [P4.2 session]

**Decision first (D065)** — the handoff's "worth a decision entry on direction before building"
is answered. C10 is classified **lateral** under D009: it binds a change set to the domain doc
and validates no doc's copy of a machine-readable fact — it is the mechanism D012 explicitly
reserves for the semantic layer. Boundary chosen: **working tree vs HEAD** (`git diff HEAD
--name-only`, index and worktree both) — the question fires before the commit is made, at the
moment the answer is cheapest; it self-corrects when code and doc land in the same change; and
it is silent on a clean tree, so existing exit bars (cinch self-check, project check-harness)
are untouched. Per-commit semantics (`git show HEAD`) and a range flag were rejected; the
pre-commit hook stays shell-calls-binary-only (D024).

**C10 implementation (cinch side)**

- `diff.go` — `checkDiffCoupling` + `gitChangedPaths` (git subprocess, 10s timeout, same shape
  as `runIntrospection`). Walks the domain layer for `owns:` lists, matches changed paths
  exactly or by directory prefix (a trailing slash like `templates/` is normalized), and warns
  **once per domain** — changed paths named, sampled past three — when the domain's doc is not
  in the change.
- `rule.go` — `ownsList()` reads the owns: list from the front-matter block (same block
  `rulePrefix` reads); `parseDomainDoc` now carries `owns []string`.
- Skip semantics mirror C5/C6/C7/C12: not a git repo, unborn HEAD, no domain layer, or no owns
  lists declared → silent. Untracked files don't fire until staged (`git diff HEAD` excludes
  them by git's design) — the question fires at the pre-commit moment.
- `diff_test.go` — 12 cases against hermetic fixture repos (`GIT_CONFIG_GLOBAL=/dev/null`):
  warns / staged counts / doc-in-same-change silent / unrelated change silent / directory
  owns entry / doc-only change silent / not-a-repo / no-commits / clean tree / no domain layer
  / no owns list / one-warn-per-domain.
- cinch's own `harness.md` owns list grew `rule.go`, `diff.go`, `introspect.go` — the harness
  domain now names its whole checker layer, and the same change that shipped C10 answered the
  question C10 itself asked about the uncommitted `check.go` edit.

**Verification**

- cinch: `make test` (vet + go test) green; self-check exit 0 with the two known C11 warns
  (C10 silent — clean tree). During the session the self-check asked the C10 question about
  the session's own uncommitted checker change: warn-level, by design, answered by the
  harness.md owns update in the same change.
- project_deltadocs: `make check-harness` exit 0, unchanged (two designed C6/C7 warns). A
  scratch content change to `backend/handlers/tab.go` fired one C10 warn per owning domain
  (signatures, tabs) and was reverted — repo left clean.
- **Decisions logged:** D065 (C10 semantics: lateral classification, working-tree boundary,
  warn-per-domain, skip semantics), E008 (session event). Roadmap §P4.2 and the P4 exit line
  annotated.

## Not done — P4 status

1. **Derivability gate (4.3, [project])** — the admission test ("can an agent recover this from
   source?") as an explicit write-time gate in the `doc-philosophy.md` template. Not started.
2. **optimize-docs deletion mandate (4.4, [project])** — must *remove* on first run, applying
   4.3 retroactively. Not started.
3. **O005** (org vs personal repo) — still open; dormant until the repo is pushed.

## Next session

The roadmap's session-loads table for the rest of P4:

> roadmap §P4.3/4.4 · `domain/overview.md` · `doc-philosophy.md` · D012 D013 — the project side
> is the derivability gate and optimize-docs' deletion mandate.

The authoring half lands in cinch's templates (doc-philosophy.md gains the one-question
admission gate; optimize-docs.md gains a pruning pass that deletes rather than refreshes),
both consumers re-render (C2), and then the project-side run actually deletes something —
the roadmap's P4 exit bar. Nothing in 4.3/4.4 is blocked.

## Notes for future work

- **C10's boundary is `git diff HEAD`, so its scope is the working set, not history.** The
  roadmap's "a commit touching..." is served at the moment before the commit exists. This is
  why a clean committed tree is always silent — `cinch check` is unchanged as a CI gate.
- **Untracked files are invisible until staged.** A brand-new file under an owns path asks the
  question once `git add`ed (or committed). If the missing case ever bites, the extension
  point is `git ls-files --others --exclude-standard`; not needed yet.
- **The session's own check asked the session's own question.** The C10 warn on the
  uncommitted `check.go` edit is the loop working as designed: warn-level, human answers "no
  rule changed", and the owns-list update landed in the same change — the cheap fix path.
- **`owns:` entries may be files or directories** (`backend/tab.go` vs `templates/`); the
  matcher normalizes the trailing slash. Renames surface by new path only (`--name-only`).
- **Next session's deletion targets live in the project's authored docs, not
  `.agent/workflows/`** (generated, C2-guarded). The doc-philosophy/optimize-docs edits are
  template edits in cinch, then `make render` + `make docs-index` in both consumers.
