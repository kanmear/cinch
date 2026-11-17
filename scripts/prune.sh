#!/bin/sh
# harness-template v2
# scripts/prune.sh [--confirm]: delete local topic branches whose squash already landed on main.
# Managed by the harness template, never edited in a repo.
#
# A branch has landed when merging it into main would change nothing: `git merge-tree
# --write-tree main <branch>` succeeds and its tree equals main's. A conflicting merge-tree
# (exit 1) means not landed. main and the checked-out branch are never considered. Without
# --confirm it only lists; with --confirm it deletes with `git branch -D` (a squash leaves the
# branch unmerged as far as `git branch -d` can tell). A branch that is just level with main
# counts as landed.
set -eu

usage() {
	echo "usage: scripts/prune.sh [--confirm]" >&2
	exit 2
}

confirm=0
case "$#:${1:-}" in
0:) ;;
1:--confirm) confirm=1 ;;
*) usage ;;
esac

if ! git rev-parse -q --verify refs/heads/main >/dev/null; then
	echo "prune: no local 'main' branch." >&2
	exit 1
fi
main_tree=$(git rev-parse 'main^{tree}')
current=$(git symbolic-ref --short -q HEAD || true)

landed=""
for b in $(git for-each-ref --format='%(refname:short)' refs/heads/); do
	[ "$b" = main ] && continue
	[ "$b" = "$current" ] && continue
	if out=$(git merge-tree --write-tree main "$b" 2>/dev/null); then
		tree=$(printf '%s\n' "$out" | head -n 1)
		if [ "$tree" = "$main_tree" ]; then
			landed="$landed $b"
		fi
	fi
done

if [ -z "$landed" ]; then
	echo "prune: no landed branches."
	exit 0
fi

failed=0
for b in $landed; do
	if [ "$confirm" -eq 1 ]; then
		if git branch -D "$b" >/dev/null; then
			echo "deleted $b"
		else
			echo "prune: could not delete $b" >&2
			failed=1
		fi
	else
		echo "landed  $b"
	fi
done
if [ "$confirm" -eq 0 ]; then
	echo "prune: nothing deleted; delete them with: scripts/prune.sh --confirm"
fi
exit "$failed"
