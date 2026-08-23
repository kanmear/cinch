#!/bin/sh

git_dir=$(git rev-parse --git-dir)
lock_file="$git_dir/.version-hook-running"

if [ -f "$lock_file" ]; then
	exit 0
fi

branch=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ "$branch" != "dev" ]; then
	exit 0
fi

parent_count=$(git rev-list --parents -1 HEAD | wc -w)
if [ "$parent_count" -gt 2 ]; then
	exit 0
fi

msg=$(git log -1 --pretty=%s)

bump=""
case "$msg" in
"merge: fix/"* | "merge: refactor/"*)
	bump="patch"
	;;
"merge: feature/"*)
	bump="minor"
	;;
esac

if [ -z "$bump" ] && printf '%s\n' "$msg" | grep -Eq '^(fix|refactor)( \[[^]]+\])?: .+'; then
	bump="hotfix"
fi

if [ -z "$bump" ]; then
	exit 0
fi

if ! git rev-parse HEAD^ >/dev/null 2>&1; then
	exit 0
fi

changed=$(git diff --name-only HEAD^ HEAD)
relevant=false
while IFS= read -r f; do
	case "$f" in
	main.go | internal/* | Makefile) relevant=true ;;
	esac
done <<EOF
$changed
EOF

if [ "$relevant" != true ]; then
	exit 0
fi

scripts/update-version.sh "$bump"
git add internal/version/major internal/version/minor internal/version/patch internal/version/hotfix

touch "$lock_file"
git commit --amend --no-edit --no-verify
rm -f "$lock_file"
