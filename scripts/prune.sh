#!/bin/sh
# harness-template v3
# scripts/prune.sh [--confirm]: delete local topic branches whose squash already landed on main.
# Managed by the harness template, never edited in a repo.
#
# A branch has landed when either test passes, tried in this order:
#   1. no diff: merging it into main would change nothing. `git merge-tree --write-tree main
#      <branch>` succeeds and its tree equals main's. A branch level with main counts here.
#   2. merge commit: main has a commit whose whole subject is exactly `merge: <branch>` (what
#      every trunk squash writes; compared as a fixed string, so `merge: <branch>x` is not it),
#      and the branch tip's committer time is not later than that commit's. The first test
#      fails once main edits the squashed files later; this one still sees the squash. The
#      time condition keeps a reused branch name safe: a branch that carries newer commits
#      than its old merge commit holds unlanded work and is kept. With several `merge:
#      <branch>` commits, the newest decides.
# Branches from before the trunk flow have no `merge:` commit and stay hand work. main and the
# checked-out branch are never considered. Without --confirm it only lists, and says which
# test matched (`landed  <b>  (no diff)` or `landed  <b>  (merge <short-sha>)`); with --confirm
# it deletes with `git branch -D` (a squash leaves the branch unmerged as far as `git branch
# -d` can tell).
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
# One pass over main: "<committer time> <short sha> <subject>" of every `merge:` commit.
merges=$(git log refs/heads/main --format='%ct %h %s' | grep -F ' merge: ' || true)

# landed: space-separated "<branch>:<reason>", reason `d` (no diff) or a short sha (merge commit).
landed=""
for e in $(git for-each-ref --format='%(refname:lstrip=2):%(committerdate:unix)' refs/heads/); do
	b=${e%:*}
	tip=${e##*:}
	[ "$b" = main ] && continue
	[ "$b" = "$current" ] && continue
	if out=$(git merge-tree --write-tree main "$b" 2>/dev/null); then
		tree=$(printf '%s\n' "$out" | head -n 1)
		if [ "$tree" = "$main_tree" ]; then
			landed="$landed $b:d"
			continue
		fi
	fi
	# the newest `merge: <b>` commit on main: "<time> <sha>", empty when there is none
	hit=$(printf '%s\n' "$merges" | WANT="merge: $b" awk '
		BEGIN { want = ENVIRON["WANT"] }
		{
			s = $0
			sub(/^[0-9]+ [^ ]+ /, "", s)
			if (s == want && (!found || $1 + 0 > t + 0)) { found = 1; t = $1; sha = $2 }
		}
		END { if (found) print t, sha }')
	[ -n "$hit" ] || continue
	if [ "$tip" -le "${hit% *}" ]; then
		landed="$landed $b:${hit#* }"
	fi
done

if [ -z "$landed" ]; then
	echo "prune: no landed branches."
	exit 0
fi

failed=0
for e in $landed; do
	b=${e%%:*}
	why=${e#*:}
	if [ "$confirm" -eq 1 ]; then
		if git branch -D "$b" >/dev/null; then
			echo "deleted $b"
		else
			echo "prune: could not delete $b" >&2
			failed=1
		fi
	else
		if [ "$why" = d ]; then
			echo "landed  $b  (no diff)"
		else
			echo "landed  $b  (merge $why)"
		fi
	fi
done
if [ "$confirm" -eq 0 ]; then
	echo "prune: nothing deleted; delete them with: scripts/prune.sh --confirm"
fi
exit "$failed"
