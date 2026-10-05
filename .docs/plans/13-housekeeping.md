# 13. Housekeeping: link parser, one prompt, stable output order, docs resync

**Status: shipped in `1bcd33c` (`chore/housekeeping`). Kept for provenance.** Written as a
self-contained hand-off plan; depended on nothing.

## Part A: the links check ignores inline code and accepts link titles

`checkLinksInFile` (`internal/cinch/links.go`) ran `mdLinkRe` (`\[[^\]]*\]\(([^)]+)\)`) on
every line outside fenced code blocks, which gave two false findings:

1. Link syntax inside inline code counted as a link: `05-navigability-check-class.md`
   reported `link target does not resolve: path` for a line containing `` `[text](path)` ``.
2. A link with a title, `[a](b.md "Title")`, captured `b.md "Title"` as the target.

Change: a `stripInlineCode` pass following CommonMark (a run of N backticks opens a span
closed by the next run of exactly N; contents become spaces so offsets don't shift; an
unclosed run is left alone), applied before `mdLinkRe` (so `buildDocLinks` is fixed too).
Targets are normalized after `TrimSpace`: `<…>` unwrapped, otherwise cut at the first
whitespace to drop a title.

Tests: single- and double-backtick spans yield no links; a titled link resolves; an
angle-bracket target resolves; an unclosed backtick before a real link still checks it.

## Part B: one yes/no prompt, stdin through a single reader

Three copies of the prompt existed (`promptYes` in `main.go`, `askYesNo` in `init.go` —
which silently rejected `yes` — and `promptYesNo` in `selfupdate.go`), and `readLine`
built a fresh `bufio.Scanner` over `os.Stdin` per call, so piped input was swallowed by the
first scanner. Change: `internal/output` gains a package-level reader, `ReadLine()` and
`AskYesNo(prompt)` (accepts `y`/`yes`, any case), written against an unexported
`*bufio.Reader` for testing; all copies replaced with visible output kept byte-identical.

## Part C: check status lines print in a fixed order

`runChecks` printed `output.CheckStatus` as goroutines finished. Change: collect results,
then print in launch order (`links, rules, index, generated, commit, core, hooks`), held in
one package-level slice used for both launching and printing (`checkLaunchOrder`).

## Part D: docs resync

Status column of `00-overview.md` brought up to date for items 2, 3, 5 and 6; status lines
added under the titles of 02, 03 and 06; README "What this doesn't prove" gained the
paragraph on rules nobody wrote — a behavior the docs never state has no ID to check, so
`cinch check` stays green.
