# project_deltadocs — the general commit-message convention, and cinch's `commit.pattern`

Status: **proposed** — not started.

Target repo: `project_deltadocs` (planned here per the "all in cinch plans dir"
decision).

## Context

The repo documents a general commit convention at `.agent/conventions.md:7-14`:

```
<type> [<scope>]: <code location>(optional) - <description>
```
Types: `feat | fix | ui | docs | style | refactor | test | chore | perf`
Scopes: `back | front | shared | docs | skills`

But **nothing enforces the non-squash case.** The only enforced commit rule is the
squash-merge window (dev/main, `merge: <branch>`), via
`scripts/check-squash-merge-msg.sh` dispatched by the `commit-msg` manifest entry.
Meanwhile cinch ships a native `commit.pattern` mechanism (`commit.go`) that the
consumer **does not use** — no `commit.pattern` key in `cinch.yml`.

The wrinkle: cinch's `commit.pattern` is a *single regex over the subject line*,
applied by the `commit-msg` hook to whatever message is in progress. But the
consumer's two message shapes are fundamentally different:
- ordinary commits: `<type> [<scope>]: …`
- squash merges onto dev/main: `merge: <branch-name>/<name>`

A single regex that matches *both* is possible only as a messy alternation that
is guaranteed to be wrong in one branch context, and neither cinch nor the script
distinguishes "this is an ordinary commit" from "this is a squash" in a
single-subject check (the script does it by peeking at `SQUASH_MSG`, a git-internal
window, which cinch does not replicate).

## Decision needed — who enforces the general convention, and how

- **A — Leave non-squash commits unenforced (status quo), do nothing.**
  Document why (the flag in `cinch-0.1.0.md` #3-style reasoning: a fuzzy or
  over-broad regex is a proxy). Cons: the documented convention stays a silent
  hope — the exact drift cinch exists to prevent.
- **B — Add a `commit.pattern` only for ordinary commits, with squash carved out.
  cinch does not support "except when message starts with `merge:`" — but there's
  a clean trick:** a pattern could never cover both, so instead *decide the
  squash case is already handled by the script and let `commit.pattern` cover the
  non-squash shape* — but cinch's commit-msg hook application has no "skip if
  merge:" branch, so B would *also* fire on `merge: …` messages inside the
  squash window and reject them. **B is therefore not valid as-is** unless cinch
  learns context (see D).
- **C — Tighten the script to also check ordinary commits, outside squash.** The
  consumer's script already has the `SQUASH_MSG`-window gate; extend it to, when
  NOT in a squash window, validate the `<type> [<scope>]: …` shape on regular
  commits (author-facing). This keeps the *branch/window context* in the script
  (where it already lives and works) while newly enforcing the documented
  ordinary-commit convention. It's a project script, so no cinch change — but it
  is documenting/enforcing outside cinch's `commit.pattern`, which some may read
  as "own rent".
- **D — Enhance cinch itself: a `commit.pattern` with branch/window context.**
  e.g. support per-branch or "squash-aware" patterns. Larger cinch-core change;
  cleanest conceptually (convention lives in the manifest, not a script), but
  grows the schema and the `commit` check's semantics. Separate cinch plan if
  wanted.

Recommendation: **C now** (consumer-only, enforces the documented convention,
low cost) *or* **D** if the user prefers the convention live in cinch's manifest.
A is the honest-defer; B is invalid as written.

## Step 1 (under C) — extend `scripts/check-squash-merge-msg.sh`

- When `$GIT_DIR/SQUASH_MSG` is absent (ordinary commit), validate the subject
  against `<type> [<scope>]: …` (`\b(feat|fix|ui|docs|style|refactor|test|chore|perf)\b`
  + optional `[scope]`) and reject with a message naming the convention + citing
  `conventions.md § Git`.
- Preserve the existing dev/main squash behavior exactly (regression-guard with
  the fixtures from `hook-commit-msg-args.md`).
- Sync with `hook-commit-msg-args.md` if that plan lands first: read the message
  from `$1` when given, else `$GIT_DIR/COMMIT_EDITMSG`.

## Step 1' (under D, separate cinch plan) — `commit.pattern` branch context

Describe the cinch-side change here as a pointer to a dedicated plan, not
implementation detail.

---

## Verification

- C: an ordinary commit with `nope: whatever` → blocked; `<type> [<scope>]: …`
  → allowed; a squash merge onto dev with `merge: feature/…` → still requires the
  squash shape; normal commits on feature branches enforce the general shape.
- `cinch check` clean; no change to the squash-only behavior.

### Critical files

- `project_deltadocs/scripts/check-squash-merge-msg.sh`
- `project_deltadocs/cinch.yml` (only under D)
- `project_deltadocs/.agent/conventions.md`
- (D only) cinch `internal/cinch/commit.go`, `manifest.go`

### Relationship to other plans

**Must consider `hook-commit-msg-args.md`** (both touch
`check-squash-merge-msg.sh`'s message-path input) — sequence together.
Under **D**, this becomes a cinch-core plan and should move out to the cinch
plans set; under **C** it stays consumer-scoped and independent.
