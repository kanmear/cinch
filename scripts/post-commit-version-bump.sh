#!/bin/sh
# harness-template v3
# scripts/post-commit-version-bump.sh <pattern>...: after a squash-merge lands on main, bump the
# version and fold the change into that commit with an amend. Managed by the harness template,
# never edited in a repo: the version-relevant paths arrive as arguments.
#
# Registered under hooks.post-commit in cinch.yml, e.g.
#   run: scripts/post-commit-version-bump.sh '*.go' go.mod go.sum
# cinch runs the string through `sh -c`, so quote the globs there. Each argument is a `case`
# pattern matched against the files the commit changed (`*` also crosses `/`); the version bumps
# only if one matches, so a docs-only or tooling-only merge never bumps.
#
# Acts only on branch `main`, on a single-parent commit whose subject is `merge: <prefix>/...`
# (the squash made by `make merge`; see .docs/git-conventions.md, § Version bump table):
#   feature/                      minor
#   fix/ refactor/ perf/ ui/      patch
#   chore/ docs/ and the rest     none
# Major is bumped by hand (`make version-major`). The bump is `scripts/version.sh <bump>`; the
# file it owns (`scripts/version.sh file`) is added and the commit amended with
# --no-edit --no-verify. With no arguments it prints a usage line and exits 0: a post-commit
# hook cannot block a commit.

# GUI git clients and non-login shells don't source shell rc files, so mise's
# shims, bun, cinch and go can be missing from PATH.
PATH="$HOME/.local/share/mise/shims:$HOME/.bun/bin:$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin:$PATH"
export PATH

if [ "$#" -eq 0 ]; then
	echo "post-commit-version-bump: no path patterns given, nothing bumps. usage: scripts/post-commit-version-bump.sh '<pattern>'..." >&2
	exit 0
fi

git_dir=$(git rev-parse --git-dir)
lock_file="$git_dir/.version-hook-running"

# Loop guard: the amend below re-triggers post-commit; bail on reentry.
if [ -f "$lock_file" ]; then
	exit 0
fi

branch=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ "$branch" != "main" ]; then
	exit 0
fi

# Exactly one parent: the squash commit. A root commit (1 word) and a merge commit (3+) never bump.
if [ "$(git rev-list --parents -1 HEAD | wc -w)" -ne 2 ]; then
	exit 0
fi

# The squash's SQUASH_MSG is already gone by now (git removes it before post-commit), so the
# subject is the only signal.
msg=$(git log -1 --pretty=%s)
case "$msg" in
"merge: feature/"*) bump="minor" ;;
"merge: fix/"* | "merge: refactor/"* | "merge: perf/"* | "merge: ui/"*) bump="patch" ;;
*) exit 0 ;;
esac

changed=$(git diff --name-only HEAD^ HEAD)

# touches <pattern>...: does any changed file match one of the case patterns?
touches() {
	while IFS= read -r f; do
		for p in "$@"; do
			# shellcheck disable=SC2254
			case "$f" in
			$p) return 0 ;;
			esac
		done
	done <<CHANGED
$changed
CHANGED
	return 1
}

touches "$@" || exit 0

cd "$(git rev-parse --show-toplevel)" || exit 0
scripts/version.sh "$bump" || exit 0
version_path=$(scripts/version.sh file) || exit 0
git add -- "$version_path"

touch "$lock_file"
trap 'rm -f "$lock_file"' EXIT
git commit --amend --no-edit --no-verify
