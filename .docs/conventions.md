# Go conventions

Style rules for the Go code under `internal/` and `main.go`. Written down once
the codebase had accumulated enough same-concept-different-style drift
(`Finding` vs `checkResult`, `UsageErr` vs `ColorizeError`, a `Report` struct
declared in a different spot in each file, ...) that a new file could no
longer tell which shape to copy.

## Exported vs. unexported

Export an identifier — type, func, var, const, or struct field — only if
something outside its package needs it. Check this concretely: `internal/cinch`
is consumed only by `main.go`, and `main.go` only ever touches `CmdCheck`,
`CmdInit`, `CmdRender`, `CmdHook`, `CmdWorkflows`, `CmdWorkflow`, `CmdIndex`,
`CmdIgnores`, `ResolveDocsRoot`, `IsGitRepo`, `ManifestExists`, and `Version`.
Everything else stays unexported.

A struct's field casing follows the struct's own exported-ness — an unexported
type gets unexported fields, since nothing outside the package can reach them
either way. Don't capitalize a field just because the concept "feels" public
(e.g. `finding`, `checkResult`, `manifest` are all unexported types with all
lowercase fields, even though `finding`/`checkResult` are the shape of the
whole check pipeline's output).

## Struct placement & field order

Types are declared together, near the top of the file, right after any
package-level `const`/`var` blocks — not interleaved with the functions that
use them, and not buried after unrelated helpers further down. If a file's
main type conceptually "belongs" to the top (e.g. a `xReport` result type),
put it there even if it's built up over many lines of logic below.

Field order: classification/identity first, then location (`file`/`line`),
then payload, then optional flags last. Example: `finding{check, level, file,
line, message}`, `ruleItem{id, file, line, text, hasIgnore, ignoreReason,
ignoreLine}`.

## Naming

**`repoRoot` / `docsRoot`.** A parameter holding the repo root is always named
`repoRoot`; one holding the docs root is always named `docsRoot` — never a bare
`root` for a docs path, never `docsDir`. When a function takes both, the order
is `(repoRoot, docsRoot)`.

**Error-flavored function names use the `Error` suffix, not `Err`** —
`output.UsageError`, not `UsageErr`. `err` stays the idiomatic local-variable
name for a value of type `error`; this rule is only about naming a function
after the error-shaped thing it produces or prints.

**"check" functions.** A `checkX` function is the entry point for one check;
it either returns `checkResult` directly, or (when it needs to report extra
counts, like `checkLinks`/`checkRules`) returns its own `xReport` that a
sibling `xCheckResult` converts to `checkResult`. Don't reuse "check" or a
check's name for something unrelated — `preCommitChecks`/`commitMsgChecks`
name the git-hook-event check bundles precisely because `hooksCheck*` would
collide with `checkHooks` (which checks whether git hooks are *activated*, a
different concept entirely).

## Variable naming

Spell out an abbreviated local variable, parameter, or struct field to its
full word — `buffer`/`temp`/`lineNumber`, not `buf`/`tmp`/`lineNo`. If a
concept already has an established fully-spelled form elsewhere in the
codebase (`message`, `path`, `pattern`, `result`, `report`, `directory`), a
clipped local holding the same kind of value matches it rather than
inventing a shorter alias — e.g. `messageFile` (not `msgFile`) because
`finding.message` is already spelled out; `commitPattern` (not `commitPat`)
because `pattern` is already spelled out everywhere else.

Exceptions — the small set of universal Go short-name idioms stay short:
`err`, `ok`, loop counters (`i`, `j`, `n`), single-letter receivers (`m
*manifest`), type-initial short-lived params (`r io.Reader`, `w`, `f
*os.File`, `d` for a `DirEntry`), `fn` for a function-value param, `a`/`b` in
sort comparators, `re *regexp.Regexp`, `b strings.Builder`. `repo`/`repoRoot`
are in the same tier — accepted git-tooling vocabulary, not a violation of
the spell-it-out rule.

## Imports

One grouped `import (...)` block per file — stdlib first, then a blank line,
then third-party/local. Never a bare `import "..."` line.

## Quoting

Single quotes for CLI command references inside user-facing messages —
`'cinch render'`, not `` `cinch render` ``.

## Error construction

Use `fmt.Errorf("...: %w", err)` when returning a real `error` a caller might
want to unwrap. Use `err.Error()` (concatenation or `%s`) when embedding into a
plain string field like `finding.message` — there's no error chain to
preserve in a display string, so `%w` doesn't apply there. Prefer
`fmt.Sprintf` over `+` concatenation once more than one value is interpolated.

## Regexp, var, and const declarations

Package-level `regexp.MustCompile` for a fixed pattern; inline
`regexp.Compile` inside the function for a pattern that comes from
`cinch.yml` or another runtime source (`commit.go`, `retirement.go`) — that's
a real distinction, not drift.

Group two or more package-level declarations of the same kind into one
`var (...)`/`const (...)` block; a lone declaration of its kind stays a
standalone line.

## Doc comments

No doc comment is required on an exported identifier. Add one only when the
WHY is genuinely non-obvious — a hidden constraint, a subtle invariant, the
reason a return shape looks odd. `manifestVar`, `manifestSetting`, `relTo`,
and `sortedKeys` are the model: short, WHY-focused, on functions whose
behavior isn't self-evident from the signature.

## Tests

Table-driven tests always use `t.Run(name, ...)` subtests, even for a short
table — a bare loop hides which case failed.

Don't re-implement a test helper that already exists in the package (e.g. the
scratch-git-repo helper `gitTestHelper`) — call it.

## File organization

Group by feature, not strictly one function per file — a file can hold a
whole feature's parsing, scanning, and checking logic (`rules.go` does, and
that's fine). What doesn't belong in a feature file is a genuinely generic,
unrelated helper: path utilities live in `scan.go`, semver parsing lives in
`semver.go`, not folded into `paths.go`/`pin.go` because that's where the one
caller happened to be.

Every `checkX` function should eventually have a `_test.go` covering it. Not
all of them do yet (`generated.go`, `retirement.go`) — new checks should ship
with tests; backfilling the gap on existing ones is tracked separately, not a
license to skip tests on new code.
