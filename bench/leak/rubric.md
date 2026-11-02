You are grading one rule from a software project's rule corpus against one invariant. The invariant
is the security property a developer's change had to keep; the rule is a candidate text someone
wrote into the project's documentation. You do not have the repository and you do not need it:
decide only whether the rule, as written, says what the invariant says.

## The invariant

{{invariant}}

## The candidate rule

{{rule}}

## How to grade

Give exactly one verdict:

- **STATES**: the rule asserts the invariant. It names the same restriction (who may or may not do
  or receive what) with the same actor, object and condition. A more general rule that covers the
  invariant counts (for example a rule about every category-scoped event, when the invariant is
  about one of them, or a rule about every project-scoped mutation by ID, when the invariant is
  about renaming a category). A stricter rule that implies the invariant counts. Wording does not
  matter; an equivalent statement does.
- **PARTIAL**: the rule points the right way but leaves out the actor, the object or the condition
  that makes it the invariant, so that someone following only this rule could still break the
  invariant. A rule that only says "respect permissions", "validate IDs" or "do not leak" is
  PARTIAL at best.
- **ABSENT**: the rule is about something else, or it contradicts the invariant, or it licenses the
  opposite behavior. A rule that restricts a neighboring thing (another event, another endpoint,
  another role) without covering the invariant's case is ABSENT.

### Read polarity, not vocabulary

Do not grade by shared words. Decide what the rule asserts and whether that is what the invariant
asserts. Negations, exceptions and conditions decide the verdict:

- "Inviting an account again rotates its link" and "a link is issued only to a caller who manages
  the account's projects" share words and say different things.
- A rule that lists the invariant's case among the cases it *permits* does not state a restriction.
- A hedge that lists both the invariant's behavior and its opposite as possibilities is not STATES.

Ignore rule IDs, file names and links in the rule text. Judge the stated behavior only.

## Output format

Reply with exactly two lines and nothing else:

1. The verdict alone, in uppercase: STATES, PARTIAL or ABSENT.
2. One sentence that quotes the decisive words of the rule in double quotes and names the part of
   the invariant they match, miss or contradict.

Example of the shape (not of any real case):

PARTIAL
The rule says "reports are only visible to staff" but does not say which staff, where the invariant limits them to the report's own team.
