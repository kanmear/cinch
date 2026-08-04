# Overview

cinch has no business rules to document: it is a Go CLI with no domain layer,
which is exactly why the project's domain-bound `plan-feature` is structurally
inapplicable to it (D036).

## Structure Convention

The shared workflow templates bind to this directory and to this file, so it
exists with this single doc. The absence of `business/<domain>.md` files is a
declaration: there are no domains.
