#!/usr/bin/env bash
# Cut a dev -> main release: tag cinch (whose version changed) and fast-forward
# dev back up to main.
#
# Run this AFTER the release merge is committed on main:
#
#   git checkout main
#   git merge --no-ff dev -m "merge: dev"   # resolve conflicts if any, then commit
#   make release
#
# The --no-ff merge stays manual on purpose (you want to see any conflicts);
# this script only performs the mechanical, near-permanent tail — creating the
# release tag and fast-forwarding dev. See conventions/git-conventions.md.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CINCH="${CINCH:-$PWD/bin/cinch}"
if [ ! -x "$CINCH" ]; then
	echo "release: $CINCH not found — run 'make build' first." >&2
	exit 1
fi

# --- Preconditions -----------------------------------------------------------
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

# dev must be contained in main: this proves the --no-ff merge happened and that
# fast-forwarding dev up to main is a pure pointer move, not a divergence merge.
if ! git merge-base --is-ancestor dev main; then
	echo "release: 'dev' is not contained in 'main' — nothing to release, or the branches" >&2
	echo "         diverged. Run 'git merge --no-ff dev -m \"merge: dev\"' on main first," >&2
	echo "         or reconcile the divergence before releasing." >&2
	exit 1
fi

# HEAD should be the release merge commit (>=2 parents => >=3 words here).
if [ "$(git rev-list --parents -1 HEAD | wc -w)" -lt 3 ]; then
	echo "release: HEAD is not a merge commit — expected the 'merge: dev' release cut." >&2
	exit 1
fi

# --- Tag cinch if its version changed ----------------------------------------
echo "Tagging release $(git rev-parse --short HEAD):"
ver=$("$CINCH" version show)
tag="cinch-v${ver}"
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
	echo "  = ${tag} already exists — unchanged, skipping"
else
	git tag -a "${tag}" -m "cinch ${ver}"
	echo "  + ${tag}"
	created="${tag}"
fi

# --- Fast-forward dev up to main ---------------------------------------------
git branch -f dev main
echo "  -> dev fast-forwarded to main ($(git rev-parse --short main))"

echo
if [ -n "${created:-}" ]; then
	echo "Release cut. New tag: ${created}"
else
	echo "Release cut. No version changed since the last release — no new tag."
fi
echo "Push with: git push --follow-tags origin main dev"
