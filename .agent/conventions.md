# Conventions (Cross-Cutting)

## Git

**Commits:** single-line subject `type: subject`, lowercase `type` (commonly `docs`, `templates`,
`check`, `render`, `decisions`); an optional `[scope]` right after the type when the change
spans a subsystem. Prefer a single-line subject; add a short body only for a non-obvious
decision that would otherwise be lost. The user reviews and commits manually — the agent never
runs `git commit`.

**Merge-commit formats and the versioning machinery** (bump scheme, hooks, release tags) live in
`ops/git-conventions.md` — the branch model and `merge: <branch>` format are defined there, not
duplicated here.

## Code

- No comments unless a reviewer would otherwise not know why. The code is the documentation.
- Tests live next to the code they exercise; parser unit tests at the module root, CLI-level
  tests under `tests/` exercising the built binary.
