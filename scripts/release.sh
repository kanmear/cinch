#!/bin/sh

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# preconditions
branch=$(git symbolic-ref --short HEAD)
if [ "$branch" != "main" ]; then
	echo "release: must be on 'main' (currently on '$branch')." >&2
	echo "         Run 'git checkout main && git merge --no-ff dev -m \"merge: dev\"' first." >&2
	exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "release: working tree not clean — commit or stash first." >&2
	exit 1
fi

if ! git rev-parse -q --verify dev >/dev/null; then
	echo "release: no 'dev' branch found." >&2
	exit 1
fi

if ! git merge-base --is-ancestor dev main; then
	echo "release: 'dev' is not contained in 'main' — nothing to release, or the branches" >&2
	echo "         diverged. Run 'git merge --no-ff dev -m \"merge: dev\"' on main first," >&2
	echo "         or reconcile the divergence before releasing." >&2
	exit 1
fi

if [ "$(git rev-list --parents -1 HEAD | wc -w)" -lt 3 ]; then
	echo "release: HEAD is not a merge commit — expected the 'merge: dev' release cut." >&2
	exit 1
fi

# tag the release
ver=$(scripts/update-version.sh show)
tag="v${ver}"
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
	echo "release: ${tag} already exists — VERSION unchanged since the last release, nothing to tag."
else
	git tag -a "${tag}" -m "${ver}"
	echo "Tagged ${tag} at $(git rev-parse --short HEAD)"
fi

# fast-forward dev up to main
git branch -f dev main
echo "dev fast-forwarded to main ($(git rev-parse --short main))"

echo
echo "Push with: git push --follow-tags origin main dev"
