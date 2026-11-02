# Agent instructions

## Project docs

This repo's operational docs live in `.agent/`. Read them before changing behavior they describe.

- Business rules are numbered items with a bold ID like **AUTH-001**. The test that enforces a rule carries a `cinch:rule <ID>` comment.
- Changing a behavior a rule describes means updating the rule and its test together. A new invariant a reader couldn't recover from the code gets a new rule.

## Project

The fixture is a full-stack app in a monorepo: a Go backend in `backend/` and a SvelteKit frontend in `frontend/`.
