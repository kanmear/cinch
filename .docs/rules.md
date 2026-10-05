---
rule_prefix: CINCH
owns:
  - internal/cinch/checks_misc.go
  - internal/cinch/render.go
  - internal/cinch/init.go
  - internal/cinch/links.go
  - internal/cinch/staged.go
---

# Cinch rules

Behaviour cinch guarantees about itself. Each rule is bound by a
`cinch:rule` marker on the test that enforces it; `cinch check` runs on this
repository, so an unbound rule or a stray marker fails CI.

1. **CINCH-001** A rendered file edited by hand is a `generated` finding.
2. **CINCH-002** A header-stamped file the current render no longer produces
   is a finding, and `cinch render` removes it.
3. **CINCH-003** `cinch init` never overwrites a `core.hooksPath` another tool
   set.
4. **CINCH-004** Render rewrites an exact `require.cinch` pin and never
   rewrites a range pin.
5. **CINCH-005** Link syntax inside inline code is not a link.
6. **CINCH-006** The pre-commit hook checks staged content; unstaged edits
   neither block nor excuse a commit.
7. **CINCH-007** Cinch sends no repository content over the network.
   <!-- cinch:ignore: a negative property; the only network call is the release lookup in selfupdate.go, verified by review -->
