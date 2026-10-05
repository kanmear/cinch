# Rule Maintenance

Use this when adding, changing, retiring, or moving a rule.

A rule doc is any doc under `{{paths.docs}}/` with numbered `ID-NNN` items; `cinch rules --json`
lists every rule with its file, line, and markers. Updates to other docs after a code change go
through `{{paths.docs}}/workflows/docs-sync.md`.

## 1. Is it a rule?

Apply the Admission Test ([Doc Philosophy](docs-philosophy.md)). A rule states what is constrained,
not how the system reports it: "only the editor can cancel" is a rule, while "returns 404 if the
caller is not the editor" is an API detail. Keep each rule to one sentence where you can.

## 2. Search before adding

Search the rule docs for the fact. If the same doc already states it, edit it in place. If another
doc does, keep the fact in the more specific doc and replace the other copy with a link. Never
leave two copies.

## 3. Write it

- Put the rule in the doc whose rules cover the same code. That doc's `owns:` front matter names the
  code, and `cinch impact` uses it to tell a change which rules it may affect. Create a new doc only
  when no existing one fits.
- A new rule gets the doc's next free ID: its `rule_prefix` plus one past the highest number in
  use. Append it. Never renumber and never reuse an ID, because markers and plans point at IDs.
- The test that enforces a rule carries a `cinch:rule <ID>` comment above it. A rule that can't be
  tested gets `<!-- cinch:ignore: <reason> -->` directly under its item instead. The reason is
  what tells a real exception apart from a skipped test.
- When you change a rule's meaning, check that its marked test still enforces the new meaning.
- To retire a rule, keep its item, state that it is retired, and put
  `<!-- cinch:ignore: retired — <why> -->` under it. The ID stays taken.

A new rule doc:

```markdown
---
rule_prefix: <PREFIX>
owns:
  - <code paths these rules cover>
---

# <Area> Rules

1. **<PREFIX>-001** <Rule one.>
2. **<PREFIX>-002** <Rule two.>
```

The prefix must be unique across `{{paths.docs}}/` and must never change.

## 4. Fix references

After moving a rule, run `cinch index --links-to <doc>` and update the docs that pointed at its old
location. Search docs and plans for the rule's ID too. Finish with a green `cinch check`.
