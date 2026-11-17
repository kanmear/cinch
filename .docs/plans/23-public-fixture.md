# 23. A public fixture

**Tier 4 — evidence. Status: decided 2026-10-27. No fixture gets built: Budgetron, made
public, is the reproducible fixture for 25 and audit recall, and DeltaDocs stays private. The
definition of done below is what's left to do.**

## Provenance

**Benchmark** (methodology). Every agent-run result so far comes from `project_deltadocs`,
which is a private product. A benchmark built only on it can't be reproduced, and that's the
weakness [21](21-benchmark-suite.md) exists to fix.

## What was considered

1. **A sanitized slice of DeltaDocs**: permissions, signatures and realtime, with its rules,
   marked tests and enough history to replay.
2. **A purpose-built app with the same shape**, if the slice couldn't be sanitized without
   losing the shape.

## Decision

- **The DeltaDocs slice is not a slice.** Its dependencies cover nearly the whole backend.
  Realtime targeting (RT-001) resolves category permission, so it needs categories, roles,
  members and the access resolver. Tabs sit on pages: `tabs.page_id` references `files`, and
  `TabHandler` takes a `FileRepository`. Signatures gate on permission levels. Only auth,
  invites, users and rate limiting fall outside, and the tests still log in through sessions.
  The "slice" would be about 85–90% of the backend and 91 of its 93 rules. Publishing that
  also means publishing the production auth path, rewriting 350 commits of history, and
  letting future models train on the leak fix before [24](24-leak-replay.md) runs.
  Declined.
- **The purpose-built app is declined for now.** It would take days, and the leak would be
  reconstructed rather than real.
- **Budgetron becomes the public fixture** (`github.com/kanmear/budgetron`, made public by the
  owner). It's a Flutter/Dart app with 39 rules, every one carrying a test marker, and a
  one-command test run (`make-test`). It's a second stack next to DeltaDocs's Go backend,
  which helps external validity. It fits [25](25-upkeep-ablation.md) and audit recall. It
  doesn't fit 24, because it has no permissions or realtime recipients to target. Its cinch
  history starts 2026-10-06, so it's thin for [20](20-rule-growth-replay.md).
- **DeltaDocs stays private.** It runs 24 on its own and 25 as a second data point, labeled
  private and not reproducible in every table. Its harness, task prompts, hidden tests,
  scoring and logs are still published. Only the repo isn't.

## Pinning

Budgetron will keep being refactored and extended, so a benchmark never runs against its
moving tip:

- Each benchmark pins a Budgetron tag (for example `cinch-bench/upkeep-v1`), created at
  pre-registration. The harness clones that tag and refuses any other commit.
- Answer keys and hidden assertions name rule IDs and paths as they stand at that tag.
- Rerunning on a later Budgetron state takes a new tag and a new pre-registration, never an
  edit to the old key.
- Task specs avoid features the owner plans to build in Budgetron. Once one of them lands
  publicly, a rerun of that task is contaminated, so the README records the publish date.

## Verification / definition of done

- Budgetron is public. A secret scan has been run over its whole history, not just the tree.
  A filename check on 2026-10-27 found no keystores, `google-services` files, `.env` files or
  key files on any branch, but gitleaks hasn't been run yet.
- At the pinned tag, `cinch check` and `make-test` pass from a clean clone.
- Someone without access to DeltaDocs can run the 25 harness against it.
