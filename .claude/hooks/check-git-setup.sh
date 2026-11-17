#!/bin/bash
# harness-template v2
# SessionStart hook: make sure the repo's git hooks are active, and warn when the session starts on
# `main`, which it must not commit to. Cloud sessions start from a fresh clone, where
# `core.hooksPath` is unset, and are often assigned `main` as their working branch, which conflicts
# with the branch model (GIT-001 in AGENTS.md). Output lands in the session's context.
#
# Its only write is `cinch init`, and only when `core.hooksPath` is unset. A `core.hooksPath` set
# to something else is reported, never changed, because it is the owner's choice. Managed by the
# harness template: byte-identical in every repo.

# Outside a work tree there is nothing to set up, and `cinch init` would scaffold a consumer there.
top=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$top" || exit 0

hooks_path=$(git config --get core.hooksPath)
if [ -z "$hooks_path" ]; then
	if command -v cinch >/dev/null 2>&1; then
		if cinch init >/dev/null 2>&1; then
			echo "[git setup ✓ hooks activated via cinch init]"
		else
			echo "[git setup ✗ cinch init failed — git hooks are NOT active]"
		fi
	else
		echo "[git setup ✗ cinch not installed — git hooks are NOT active, so nothing checks commit messages."
		echo " Install cinch and run cinch init; until then follow GIT-001 in AGENTS.md and run the"
		echo " relevant make check target yourself before each commit.]"
	fi
elif [ "$hooks_path" != ".githooks" ]; then
	echo "[git setup ⚠ core.hooksPath is '$hooks_path', not '.githooks' — cinch's hooks may not run here."
	echo " Not changing it; tell the user if commits are not being checked.]"
fi

branch=$(git branch --show-current 2>/dev/null)
if [ "$branch" = "main" ]; then
	echo "[git setup ⚠ session is on 'main' — never commit to it directly (GIT-001 in AGENTS.md)."
	echo " Create a feature/, fix/, refactor/, perf/, ui/, chore/ or docs/ branch off main and work there,"
	echo " even if the session was told to develop on 'main'; say so to the user.]"
fi
exit 0
