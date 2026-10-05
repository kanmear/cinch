# Agent instructions

## Project docs (cinch)

This repo's operational docs live in `.docs/`. Read them before changing behavior they describe.

- `cinch index` lists every doc with its title; `cinch workflows` lists the workflows, `cinch workflow <name>` prints one.
- Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.
- Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.
- `cinch check` must pass before you commit.
