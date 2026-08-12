# Documentation Sync

Update `{{paths.docs}}/` documentation to reflect recent code changes.
- **Note:** `cinch index` reflects the doc structure live — nothing to regenerate or stage when a
  doc is added, removed, renamed, or retitled. AGENTS.md itself only changes when a top-level
  reading-guide entry point changes.

Read [Doc Philosophy](docs-philosophy.md) before proceeding — it defines what to document and what to skip.

## Usage

Automatically update docs based on recent changes, including:
- Regular doc updates (endpoints, models, config, etc.)
- Quirks/gotchas discovered → routed to the section that exists for that service (see §4)
- Recurring issues fixed → automatically added to troubleshooting.md

## Instructions

### 1. Gather context

First, determine if there are unstaged changes:

```bash
git status --short -uall
```

**If there are unstaged changes (M, ??, D, etc.):**
- Use `git status --short -uall` to see all changed files (modifications, new files including those in new directories, deletions)
- This is the context of what was changed

**If there are no unstaged changes:**
- Use `git diff --name-only HEAD~1` to see files from the last commit
- Use `git log -1 --oneline` to see the commit message
- This is the context of what was changed

Read the changed source files to understand what's doc-worthy.

### 2. Decide if docs need updating

**Ask yourself: Does this change affect information that can't be derived from the code?**

Changes that **DO** warrant doc updates:
- New API endpoints or modified endpoint contracts
- New/changed environment variables or configuration
- Architectural decisions (new middleware layer, state management approach)
- New commands added to Makefile or package.json scripts
- File structure reorganization (new directories, moved components)
- Non-obvious patterns or workarounds discovered
- Project-specific conventions established

Changes that **DON'T** warrant doc updates:
- Refactoring that doesn't change architecture (renamed variables, split functions)
- Bug fixes that don't reveal a gotcha worth documenting
- Adding standard CRUD operations following existing patterns
- Styling changes (CSS, layout tweaks, color updates)
- Implementation details visible in the code (how a function works internally)
- Performance optimizations that don't change interfaces

**When in doubt:** If an agent could figure it out by reading the changed source files, don't document it.

### 3. Analyze changes and classify them

Read the changed source files and classify each change:

**Regular Documentation Updates** (most changes):
- New API endpoints or modified endpoint contracts → `api/[domain].md`
- New/changed data models → `models/[entity].md`
- Environment variables, ports, commands → `cinch_manifest`
- File structure changes → relevant `architecture.md`
- Architectural decisions → `conventions.md`

**Quirks/Gotchas:**
Detect these by looking for:
- **Non-standard workarounds** — unusual patterns that look intentional but aren't self-explanatory
  from the name alone (e.g. a project-specific helper whose purpose only becomes clear from its call
  sites — document what it does and point at the relevant `<service>/conventions.md` section)
- **Framework-specific oddities** — e.g. a config or env file that must be loaded in a specific
  order, where the tooling doesn't enforce that order automatically
- **Security-related patterns** — e.g. a serialization/marshaling tag that must be set on a
  sensitive field to keep it out of API responses
- **Integration quirks** — e.g. a required client option that isn't the library's default and
  silently breaks a cross-cutting concern (auth, CORS) if omitted
- **Commit messages mentioning** "workaround", "hack", "gotcha", "weirdly"

**Common Issues** (add to troubleshooting.md):
Detect these by looking for:
- **Bug fixes that required non-obvious diagnosis** — If it took >30min to figure out, document it
- **Commit messages mentioning** "fix", "broken", "wasn't working", "issue"
- **Environment-specific problems** — Docker, database, or build issues
- **Dependency conflicts or version-specific bugs**
- **Race conditions or timing issues** discovered and fixed
- **Only document if the issue could recur** — Skip one-off typos or obvious mistakes

### 4. Route to appropriate files

**Routing table:**

| Change kind | Destination |
| ----------- | ------------- |
| New/changed API endpoint | `api/[domain].md` |
| New/changed data model | `models/[entity].md` |
| Env var / Make-npm command / port | `cinch_manifest` |
| New directory / major file move | relevant `architecture.md` |
| Project-wide convention or architectural decision | root `conventions.md` / relevant `architecture.md` |
| Service-specific pattern | `[service]/conventions.md` |
| Frontend quirk | the frontend layer's conventions doc (find it via `cinch index`) § Quirks and Gotchas |
| Backend quirk | the backend layer's troubleshooting doc (find it via `cinch index`) § Quirks |
| Cross-cutting quirk | root docs only if a real quirks heading exists; otherwise the closest service troubleshooting § Quirks |
| Common / recurring issue | `[service]/troubleshooting.md` § Common Issues (infra → `deployment.md`) |

**For Regular Updates:** run `cinch index`, read only the affected docs, update ONLY the
affected sections. Focus on facts: endpoint specs, config values, file locations, architectural
decisions.

**For Quirks:** format **Context** → **The Quirk** → **Why it's necessary** → **Example reference**.
Keep it brief; reference the source file for full implementation.

**For Common Issues:** format **Symptom** → **Root Cause** → **Fix** → **Prevention**. Include just
enough detail to diagnose and fix.

**Out of scope:** `{{paths.docs}}/` docs are not updated by this workflow. Domain rule changes are
managed separately via `{{paths.docs}}/workflows/docs-maintain-domain.md`.

**When to Create a New Doc File:**

Create a new file when:
- A new feature domain is added (e.g., things → `api/<resource>.md`)
- A new entity is introduced (e.g., Thing model → `models/<entity>.md`)
- A component/module has complex, non-obvious behavior that agents will need repeatedly

Place new docs alongside existing similar docs — run `cinch index` for this project's
current layout and naming conventions before creating a new file.

**Update AGENTS.md only when** a top-level reading-guide entry point changes (a new high-level doc
category / new top-level concern) — not for every new file. Individual files are covered by
`cinch index`, not by AGENTS.md.

### 5. Update docs strategically

Apply [Doc Philosophy](docs-philosophy.md): surgical edits, preserve style,
cross-reference instead of duplicating, be specific, no fluff. Prefer "See `path`" over inlining
code the codebase already demonstrates.

Classification decision rules (not covered by philosophy alone):
- **Misclassifying changes** — Don't treat every bug fix as a "common issue" (only if non-obvious
  and could recur); don't mark standard patterns as "quirks" (only actual workarounds); don't skip
  quirk detection just because code "seems normal" — if it required research to find the solution,
  it's a quirk.
- **Wrong location within `{{paths.docs}}/`** — use the routing table in §4; API specs belong in
  `api/[domain].md`, model schemas in `models/[entity].md`.

## Summary Checklist

Before updating any doc, verify:
- [ ] This information can't be derived from reading the source code
- [ ] I've correctly classified this as: regular update / quirk / common issue
- [ ] This isn't duplicating information in another doc (cross-reference instead)
- [ ] This is a fact, decision, or quirk — not generic programming advice
- [ ] This describes something that actually exists in the codebase now
- [ ] The update is surgical (minimal changes to affected sections only)
- [ ] I'm putting this in the right file according to the routing table in §4
