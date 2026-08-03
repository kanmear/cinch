# cinch — kickoff

How to go from "one repo holding a project, its harness, and a half-finished rebuild" to two repos
and a working sequence. Read once. After that, `cinch/docs/ROADMAP.md` and `cinch/decisions.jsonl`
are the live documents.

---

## 0. How to use this package

Not "drop it in and point an agent at this file." This is orientation for you; the roadmap is what
agents work from.

1. **Phase 0 by hand** (§2). It creates a *second* repo and edits the first — five minutes of shell,
   awkward to delegate because it spans two working trees.
2. **From P1 on, one agent session per phase side.** Open the relevant repo, point the session at
   `cinch/docs/ROADMAP.md` §P<n> plus the row from §4. Not this file, and not the whole roadmap.
3. **After each phase**, append an event (§5).

The roadmap is the live document. This file goes stale the moment P0 is done, and that is fine.

---

## 1. What's in this package

**`cinch/`** — becomes the new repo.

```
AGENTS.md              minimal; cinch's own harness, nothing like the project's
Makefile               build, vet, fixtures
docs/ROADMAP.md        the phased plan (v3) — the live document
docs/RATIONALE.md      full reasoning behind the phases; reference, not a session load
docs/decisions-log.md  log schema and superseding convention
decisions.jsonl        44 entries: 39 decisions, 1 open question, plus non-goals
templates/             empty — P1 fills it
fixtures/              empty — P3 fills it
*.go, go.mod           the CLI: render, index, check (wave-one checkers)
```

**`project/`** — goes into the existing repo.

```
.agent/manifest.example.yml   target manifest shape; merge, don't replace
Makefile.snippet              three targets calling ~/.cinch/bin/cinch
pre-commit.sh                 install to .git/hooks/pre-commit
```

Note what is **not** in `project/`: the roadmap, the rationale, the decisions log. Those are cinch's
design record and belong in cinch (D032). Left in `.agent/` they would enter the project's doc index
and load into project sessions.

---

## 2. Phase 0, concretely

One goal — two repos. Nothing improves here; the point is that everything after this happens in a
context window holding one concern.

```sh
# 0.1  create the repo
mkdir ~/.cinch && cd ~/.cinch && git init
cp -r <package>/cinch/. .
go mod tidy && make build
git add -A && git commit -m "cinch: initial import"

# optional: bring the workflow files' history along instead of starting clean
#   git clone <project> /tmp/cinch-seed && cd /tmp/cinch-seed
#   git filter-repo --path .agent/workflows --path .agent/doc-philosophy.md

# 0.2  strip cinch material out of the project
#      move harness-philosophy.md into cinch/docs/ too — the north stars and
#      Foundation are portable spec, not project content (D032, D037)
cd <project>
cat <package>/project/Makefile.snippet >> Makefile
cp <package>/project/.agent/manifest.example.yml .agent/
install -m755 <package>/project/pre-commit.sh .git/hooks/pre-commit

# 0.3  delete the compliance ritual
#   grep -rn "AGENTS.MD initialized\|MANDATORY\|protocol violation" .agent AGENTS.md
#   remove every hit; pure deletion, no replacement
```

**Exit:** a project session's injected context contains no cinch material; a cinch session's
contains no project domain material. `make check-harness` in the project runs and fails loudly —
correct, the manifest isn't complete yet.

---

## 3. Running a phase

**Do not plan cinch work with the project's `/plan-feature`** (D036). It carries the domain table,
stack bindings, and test taxonomy — invoking it in a cinch session reimports the material P0 exists
to separate. It is also structurally inapplicable: its opening front is domain rules plus an
interaction table, and cinch is a Go CLI with no domains and no business rules.

**P0 and P1: plan in-session.** Build sessions run a frontier model (D035), so the roadmap section
plus its *Exit* line is enough. No plan file, no `T<n>` decomposition. The decision IDs in the phase
text are constraints, not suggestions.

**P2 produces the replacement.** Extracting the domain-agnostic task primitive gives cinch a
planning workflow it can render for itself — cinch becomes the primitive's first consumer. From P3
onward, plan cinch phases with `cinch render`ed output of its own primitive. That is both a working
procedure and the strongest portability evidence available, ahead of the toy fixtures.

**During.** `make check-harness` after each meaningful change. The §4 table is a relevance filter —
keep project domain material out of cinch sessions — not a budget ceiling; the ~90k constraint
belongs to the consumer, not the workspace.

**End of a phase.** Exit criteria met and checked; an `E<n>` event appended (§5).

**One procedural rule survives the split** (D025): the *project's* `plan-feature` is replaced by a
rendered one during P1 and P2, so project-side sessions in those phases should use the current
workflow until the rendered one lands.

---

## 4. Per-phase relevance

What a session should have in front of it. A relevance filter, not a budget ceiling (D035) — the
point is keeping project domain material out of cinch sessions and vice versa.

| Phase | Side | Load |
|---|---|---|
| 0 | both | roadmap §P0 · project `AGENTS.md` · D031 D032 |
| 1 | cinch | roadmap §P1 · `render.go` `manifest.go` · one workflow as the port pilot · D005 D008 |
| 1 | project | roadmap §P1 · `manifest.example.yml` · current `manifest.yml` · D001 D010 |
| 2 | cinch | roadmap §P2 · `plan-feature` `fix-bug` `execute-plan` · philosophy §Foundation 2 |
| 3 | cinch | roadmap §P3 · `check.go` · project `manifest.yml` · D009 D011 D027 D028 D030 |
| 4 | project | roadmap §P4 · `business/overview.md` · one `business/<domain>.md` · `doc-philosophy.md` · D012 D013 D027 |
| 4 | cinch | roadmap §P4.2 · `check.go` · D009 |
| 5 | both | roadmap §P5 · `manifest.yml` · workflow list · philosophy §Forward vision · D014 D015 |

The failure this prevents is a cinch session loading the project's business rules, or a project
session loading cinch's design history.

---

## 5. Keeping the log current

`cinch/decisions.jsonl` is append-only. Never edit a line.

```json
{"id":"E001","ts":"2026-08-10","kind":"event","title":"Phase 1 landed","decision":"Render live; 3 workflows templated.","rejected":null,"why":"Needed a commands.* alias — templates read {{commands.check}} but the manifest nests under development.commands.","refs":["roadmap P1"]}
```

`why` is the field that earns the file. To reverse a decision, append one carrying
`"supersedes": "D0NN"`. To resolve O005, append a `D0NN` with `"supersedes": "O005"`.

The project gets its own separate log for project decisions (roadmap §anytime). Two logs, two
concerns — same reason as two repos.

---

## 6. Status of the shipped code

Written but **not compiled** — no Go toolchain was available where it was produced. Expect a small
fix or two on first `go build`. Roughly 600 lines of stdlib plus `yaml.v3`, implementing:

- `render` — literal substitution; an undefined variable is an error (D005). Templates resolve from
  `$CINCH_HOME/templates`, then `<binary>/../templates`, then `~/.cinch/templates`.
- `index` — H1 extraction into `.agent/index.md`.
- `check` — wave one: C1, C2, C3, C4, C5, C9, C11, C12.

Two things worth knowing before reading it: **C2 distinguishes stale from tampered** via a body hash
in the generated header, and **the manifest flattener aliases `development.commands.*` to
`commands.*`**, because the philosophy's templates say `{{commands.check}}` while the manifest nests
commands under `development`.

`templates/` and `fixtures/` are empty — P1 and P3 work.

---

## 7. Order

```
0  split the workspace         [both]    one goal: two repos
1  binding layer               [both]    unblocks everything
2  task primitive              [cinch]   unblocks 5
3  checker layer, 3 waves      [cinch]   ship C2 with the render step
4  semantic integrity          [both]    needs 1, 3
5  seams, tiering, permissions [both]    needs 1, 2

anytime  session-start signals, episodic memory   [project]
```

**O005 is the only open question** — cinch under a GitHub org or a personal repo. It blocks P0.1 and
nothing else.
