You are grading one answer to a question about a software repository. A developer asked the
question; someone answered it. You are given the rule that states the correct behavior (the key)
and the grading notes for this question. You do not have the repository and you do not need it:
grade only whether the answer states what the key says, as the notes interpret it.

## The question

{{question}}

## The answer being graded

{{answer}}

## The key (the rule that holds in this repository)

{{key}}

## Grading notes for this question

{{notes}}

## How to grade

The key may contain more than the question asks about. The notes say which of its clauses this
question tests ("Focus") and what counts as CORRECT, PARTIAL and WRONG. Clauses the notes call
"not required" never count against the answer. Where the notes and the general definitions below
disagree, follow the notes.

Give exactly one verdict:

- **CORRECT**: the answer states the substance the notes require, including every quantity,
  actor and condition the key fixes for those clauses (a number, a limit, a sign, who may do
  it, an inclusive or exclusive bound, which records are kept and which are deleted). Wording
  does not matter; an equivalent statement counts. Extra detail is fine as long as none of it
  contradicts the key.
- **PARTIAL**: the answer points the right way but misses a required quantity, actor or
  condition, or states one too vaguely to check (for example "it is rate-limited" where the key
  fixes the rate). Nothing it says contradicts the key.
- **WRONG**: the answer contradicts the key on a required clause, or invents a behavior the key
  rules out. An answer that is correct on one clause and contradicts the key on another required
  clause is WRONG.
- **ABSTAIN**: the answer says it cannot tell, or gives no answer to what was asked (it only
  describes where to look, or only hedges between options without choosing).

### Read polarity, not vocabulary

Do not grade by shared words. Decide what the answer asserts and whether that is what the key
asserts. Negations, exceptions and conditions decide the verdict:

- An answer that says "the request is never auto-cancelled" asserts the opposite of "the request
  is auto-cancelled", even though both use the word. If the key says it is not auto-cancelled, the
  first is CORRECT; if the key says it is, the first is WRONG.
- "Inclusive on both ends" and "start inclusive, end exclusive" share most of their words and
  contradict each other.
- "Treated as `none`" (a feature's fallback) and "the default is `none`" (a built-in default) are
  different claims when the key distinguishes them.
- A hedge that lists both the key's behavior and its opposite as possibilities is not CORRECT.

The answer may mention files, line numbers or placeholders such as `[ref]`. They tell you nothing
about correctness; ignore them. Judge the stated behavior only.

## Output format

Reply with exactly two lines and nothing else:

1. The verdict alone, in uppercase: CORRECT, PARTIAL, WRONG or ABSTAIN.
2. One sentence of justification that quotes the decisive words of the answer in double quotes
   and names the clause of the key they match, miss or contradict.

Example of the shape (not of any real case):

PARTIAL
The answer says "reports are throttled and never run in parallel" but gives no interval, where the key fixes "at most once an hour".
