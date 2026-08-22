#!/bin/sh

git_dir=$(git rev-parse --git-dir)

msgfile="${1:-}"
if [ -n "$msgfile" ]; then
	msg=$(head -n 1 "$msgfile")
else
	msg=$(head -n 1 "$git_dir/COMMIT_EDITMSG")
fi

if [ -f "$git_dir/SQUASH_MSG" ]; then
	branch=$(git symbolic-ref --short HEAD 2>/dev/null)
	if [ "$branch" != "dev" ]; then
		exit 0
	fi

	case "$msg" in
	"merge: feature/"* | "merge: refactor/"* | "merge: fix/"* | "merge: chore/"* | "merge: docs/"*)
		exit 0
		;;
	*)
		echo "check-squash-merge-msg: squash-merge onto dev must be 'merge: <feature|refactor|fix|chore|docs>/<name>' (got: \"$msg\")" >&2
		exit 1
		;;
	esac
fi

if [ -f "$git_dir/MERGE_HEAD" ]; then
	exit 0
fi

if ! printf '%s\n' "$msg" | grep -Eq '^(feat|fix|ui|docs|style|refactor|test|chore|perf)( \[[^]]+\])?: .+'; then
	echo "check-squash-merge-msg: commit message must be '<type> [<scope>]: <description>' (types: feat|fix|ui|docs|style|refactor|test|chore|perf; scope optional) — see .docs/git-conventions.md § Git (got: \"$msg\")" >&2
	exit 1
fi
