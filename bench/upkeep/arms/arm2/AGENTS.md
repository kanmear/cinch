# Agent instructions

## Project docs

This repo's operational docs live in `.docs/`. Read them before changing behavior they describe.

- Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.
- Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.

## Project

Budgetron is a Flutter app (ObjectBox, local-only). Read `.docs/architecture.md` and `.docs/conventions.md` first. The Flutter toolchain is at `~/dev/flutter`; run tests with `flutter test`.
