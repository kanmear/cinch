# Documentation Sync

Use this after a code change, to bring the docs that describe it back in line.

Rule changes go through `{{paths.docs}}/workflows/docs-maintain-domain.md`; this workflow covers
every other doc under `{{paths.docs}}/`. `cinch index` lists them with their titles.

## 1. Gather the change

Run `git status --short -uall`. If it lists nothing, the change is the last commit: use
`git diff --name-only HEAD~1` and `git log -1`. Read the changed source files.

## 2. Rules first

Run `cinch impact <changed files>`. For each rule it names, check whether the change altered the
behavior the rule states. If it did, update the rule and its test through the rule workflow.

## 3. Apply the Admission Test to each change

Read [Doc Philosophy](docs-philosophy.md). A change earns a doc update only if it alters something
a reader couldn't recover from the source: a contract between components, a decision and its
reason, a non-obvious convention, a gotcha. If an agent could learn it by reading the changed
files, document nothing.

## 4. Update the doc that already owns the fact

Search `{{paths.docs}}/` for the fact, or for the changed file's path, and edit that doc in place.
Keep edits surgical, match the doc's style, and write "See `<path>`" instead of copying code.
Create a new doc only when no doc covers the area, and put it next to similar docs (`cinch index`
shows the layout).

Two kinds of entry have a fixed format:

- **Quirk**: a workaround or oddity that looks wrong but is intentional. Write
  **Context** → **The quirk** → **Why it's necessary** → **Example** (`file:line`), in the
  conventions doc for the affected area.
- **Common issue**: a bug whose diagnosis was non-obvious and that could recur. Write
  **Symptom** → **Root cause** → **Fix** → **Prevention**, in the troubleshooting doc for the
  affected area. One-off typos and obvious mistakes don't qualify.

If the project has no conventions or troubleshooting doc, put the entry in the doc closest to the
affected code rather than creating a new doc for a single entry.

## 5. Check

Run `cinch check`. A removed or renamed doc shows up as a broken link, and
`cinch index --links-to <doc>` finds the docs that pointed at it.
