# Documentation Audit

Audit `.agent/` documentation for philosophy violations, bloat, and staleness.

Read `.agent/workflows/doc-philosophy.md` — every check below enforces these principles.

## Usage

Scan all `.agent/` docs and fix violations.

## Instructions

### 1. Load context
- Read `.agent/index.md` (the generated doc map) for the full doc index
- Read `manifest.yml` for the source-of-truth on env vars, ports, commands, taxonomy, paths

### 2. Audit each doc file
Read every `.md` file under `.agent/`. For each, check:

**Speculative content** (principle 5)
- References to unimplemented features, "planned" sections, example code for things that don't exist
- Check: does the described thing exist in the codebase?

**Duplication** (principle 3)
- Info that duplicates another doc or manifest.yml
- Check: is the same fact stated in two places? Has it diverged?
- Specifically watch for a value resolved from `manifest.yml` (e.g. `taxonomy.test_tiers`,
  `development.commands`) restated as prose or a table in a workflow doc instead of referenced —
  this drifts silently whenever the manifest changes

**Bloated link sections** (principle 4)
- "Related Documentation" sections that list every doc (that's what the generated `.agent/index.md` is for)
- Check: are links scoped to directly related docs only (3-7 links)?

**Code duplication** (principle 2)
- Code examples that duplicate what the codebase shows
- Check: would "See `file:function`" suffice instead of inline code?

**Generic content** (principle 1)
- Standard language/framework explanations any LLM knows
- Check: is this specific to THIS project?

**Stale references**
- Links and manifest bindings that don't resolve — mechanically covered by
  checkers: C11 checks every markdown link under `.agent/`, C4 checks every
  manifest `paths.*` binding, and rendered workflow command references are
  generated from the manifest (C2 guards the rendering). What remains is
  judgment: a hand-authored doc pointing at a command or path that moved.
- Check: do referenced paths and commands still exist?

### 3. Report findings
List violations grouped by file, with the principle violated and suggested fix.

### 4. Apply fixes
- For each violation, apply the minimal fix (trim, cross-reference, or remove)
- Re-read modified files to confirm they're still useful
