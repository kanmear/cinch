# Documentation Audit

Use this periodically to prune docs that fail the Admission Test and to fix duplication, bloat, and staleness.

Every check below enforces [Doc Philosophy](docs-philosophy.md); read it first.

## 1. Load context

Run `cinch index` for the full list of docs and their titles.

## 2. Prune first: the Admission Test, applied retroactively

Apply the Admission Test to every existing doc, not just to new content:

- Anything an agent could recover by reading the source is **removed, not trimmed and not
  refreshed**. Deletion is the fix, and in this step the corpus only shrinks.
- What survives is the residue that can't be derived: rules (IDed and guarded by `cinch check`),
  decisions and their reasons, and the generated workflows (owned by cinch).
- After pruning, run `cinch check` again. Deleting content must leave it green, not just shorter.

## 3. Audit each doc

Read every `.md` file under `{{paths.docs}}/` except `workflows/`, which cinch generates. For each,
check:

**Speculative content** (principle 5)
- References to unimplemented features, "planned" sections, examples for things that don't exist
- Check: does the described thing exist in the code?

**Duplication** (principle 3)
- A fact stated in two docs
- Check: has one copy diverged? Keep the most specific one and link to it from the other.

**Bloated link sections**
- "Related Documentation" sections that list every doc (`cinch index` does that)
- Check: are the links limited to directly related docs?

**Code duplication** (principle 2)
- Code examples that repeat what the codebase shows
- Check: would "See `<path>`" do instead?

**Generic content** (principle 1)
- Explanations of standard language or framework behavior that any LLM knows
- Check: is this specific to this project?

**Stale references**
- `cinch check` already catches broken links and markers, and `cinch render` regenerates the
  workflows. What remains is judgment: a hand-written doc naming a command or path that moved.
- Check: do the referenced paths and commands still exist?

## 4. Report findings

List violations grouped by file, with the principle each one breaks and the suggested fix. Name
everything removed in the pruning pass, and spot-check with a search that each deleted fact is
still recoverable from the source.

## 5. Apply fixes

- Content that fails the Admission Test is removed, never refreshed or trimmed.
- Content that passes but is redundant gets the smallest fix: trim it or replace it with a link.
- Re-read each modified file to confirm it is still useful.
