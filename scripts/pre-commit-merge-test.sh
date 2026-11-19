#!/bin/sh
# harness-template v3
# scripts/pre-commit-merge-test.sh [command [args...]]: pre-commit step that runs the tests only
# when the commit lands a squash-merge. `git merge --squash` leaves $GIT_DIR/SQUASH_MSG until the
# commit is made; ordinary commits get the static gate only. A plain merge (MERGE_HEAD) is not a
# way to land a branch (.docs/git-conventions.md) and is not special here. Managed by the harness
# template, never edited in a repo: the command arrives as arguments, e.g. in cinch.yml
#   run: scripts/pre-commit-merge-test.sh make test
# With no arguments it runs `make test`.
gitdir=$(git rev-parse --git-dir)
if [ -f "$gitdir/SQUASH_MSG" ]; then
	if [ "$#" -eq 0 ]; then
		exec make test
	fi
	exec "$@"
fi
echo "skipping tests: not a squash-merge commit"
