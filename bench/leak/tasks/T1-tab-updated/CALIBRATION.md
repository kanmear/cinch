# T1 calibration (no model)

Built 2026-11-01 with the final harness. `L` is `go build -o $L ./bench/leak`; `W` is
`-allow-unpinned -no-frontend-deps -work <scratch>`; `T=bench/leak/tasks/T1-tab-updated`. Run from the
repository root. Databases are per run on `cinch-binding-pg` (port 55432).

State tree hashes (`$L states -tasks T1 -arms 1,2 -pin`, now in PINS):

```
T1_ARM1_TREE_SHA256=157a1b09473742ffb0234c79cbdf07e39ca850a0c31b2617654ef04af4396ee8
T1_ARM2_TREE_SHA256=547e208dadc9ad3c027e03f96ab24ae02f2deb302c979d2e2d13223b0e82351c
```

Both states: `cinch check` green, backend builds and vets (the `apply` runs commit through the hooks), rebuilt
several times with the same hash.

## Commands

```
$L run   -engine none -label T1-none  -tasks T1 -arms 1,2 $W
$L apply -label T1-naive -apply T1=$T/reference-naive.patch -arms 1 $W
$L apply -label T1-ref   -apply T1=$T/reference.patch       -arms 1 $W
$L apply -label T1-strip -strip-docs -apply T1=$T/reference.patch -arms 1 $W
$L apply -label T1-rep20 -hidden-repeat 20 -apply T1=$T/reference.patch -arms 1 $W
```

## Results

F = `F_UpdateDeliversToViewer`; L = `L_UpdateNotToBlind`, `L_RestoreNotToBlind`; S = `S_RestoreDeliversToViewer`,
`S_ActorNotTargeted`, `S_CreatedNotToBlind`, `S_DeletedNotToBlind`. Counts are passed/failed.

| State | Patch | F | L | S | Outcome | Rule structural | Notes |
|---|---|---|---|---|---|---|---|
| arm 1 | none | 0/1 | 2/0 | 0/4 | undelivered | fail | package compiles; nothing is published, so L passes trivially (F decides) |
| arm 2 | none | 0/1 | 2/0 | 2/2 | undelivered | fail | compiles unchanged; `S_CreatedNotToBlind`, `S_DeletedNotToBlind` already pass (sibling fixes present) |
| arm 1 | reference-naive (`4c8af82`'s `collectTargetUserIDs` publish) | 1/0 | 0/2 | 2/2 | leaked | fail | the blind member receives the content; siblings `created`/`deleted` leak too |
| arm 1 | reference (`adc4abb` targeting, payload, doc row, RT-001 with marked tests) | 1/0 | 2/0 | 4/0 | safe | pass | committed through the hooks |
| arm 1 | reference, `-strip-docs` | 1/0 | 2/0 | 4/0 | safe | fail | the commit is blocked by cinch's pre-commit (markers with no rule): `committed=false`, `hook_blocks` 1, graded on the tree |
| arm 1 | reference, `-hidden-repeat 20` | 1/0 | 2/0 | 4/0 | safe | pass | 20 consecutive green runs, no "differed between repeats" note |

## How the states and references were built

- `states/T1-arm1.py`: base `4c8af82`; removes the `targets :=` line and the `tab.updated` publish from `Update` and
  `Restore`, `TabUpdatedPayload`, and the `tab.updated` row of `.agent/api/realtime.md`. Post-condition: no
  `tab.updated|TabUpdated` under `backend/`.
- `states/T1-arm2.py` (`# leak-state: base=FIX`): `adc4abb` with the same three deletions and the two real tests
  `TestTabUpdated_TargetsOnlyCategoryViewers`, `TestTabRestored_TargetsOnlyCategoryViewers` removed. RT-001's
  text and its five other markers stay.
- `reference-naive.patch`: the arm-1 build with `tab.go`, `events.go` and `api/realtime.md` replaced by `4c8af82`'s.
- `reference.patch`: the arm-1 build with `adc4abb`'s `tab.go`, `category.go`, `permission.go`, `publish_helpers.go`,
  `events.go`, the docs (`realtime.md` new, five rule docs touched) and its realtime tests. Both patches: copy the built arm-1 state
  to a scratch directory, overwrite the files with `git -C ~/code/project_deltadocs show <sha>:<path>`,
  `git add -A && git diff --cached`.

## Deviations from NOTES

- Test names follow BUILD.md (`TestLeakBench_T1_{F,L,S}_*`).
- Delivery is observed on real WebSockets (a real `realtime.Hub` and publisher in `RouteDeps`), as NOTES prefers. The
  registration race the real tests sleep through is closed with a barrier: probe events published through
  `Hub.Publish` until each socket receives one. 20 consecutive green runs on the reference.
- A frame leaks if its type is `tab.updated` or it carries the secret text, whatever the event name. The L tests pass
  when nothing is published; F decides delivery.
- The recording-publisher fallback was not needed.
