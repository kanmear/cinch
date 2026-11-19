#!/bin/sh
# harness-template v3
# scripts/release.sh [remote]: tag v<version> on main and push main and the tag. Managed by the
# harness template, never edited in a repo. The remote defaults to origin.
#
# The version is `scripts/version.sh show`. The tag is annotated. Refuses (exit 1) when not on
# main, when tracked files have staged or unstaged changes, or when a merge or squash is in
# progress. Rerunnable:
#   tag already on HEAD       says so and still pushes main and the tag
#   tag on another commit     no version-relevant change has landed since that release: nothing
#                             is tagged, main is still pushed
# See .docs/git-conventions.md (§ Release). Delivery (deploy, install) is not this script's job.
set -eu

PATH="$HOME/.local/share/mise/shims:$HOME/.bun/bin:$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin:$PATH"
export PATH

if [ "$#" -gt 1 ]; then
	echo "usage: scripts/release.sh [remote]" >&2
	exit 2
fi
remote=${1:-origin}

cd "$(git rev-parse --show-toplevel)"
git_dir=$(git rev-parse --absolute-git-dir)

branch=$(git symbolic-ref --short -q HEAD || true)
if [ "$branch" != "main" ]; then
	echo "release: must be on 'main' (currently on '${branch:-a detached HEAD}')." >&2
	exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "release: working tree not clean: commit or stash first." >&2
	exit 1
fi

if [ -f "$git_dir/MERGE_HEAD" ] || [ -f "$git_dir/SQUASH_MSG" ]; then
	echo "release: a merge is in progress: commit it or abort it first." >&2
	exit 1
fi

version=$(scripts/version.sh show)
tag="v$version"
head=$(git rev-parse HEAD)

push_tag=""
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
	tagged=$(git rev-parse "refs/tags/$tag^{commit}")
	if [ "$tagged" = "$head" ]; then
		echo "release: $tag already exists on HEAD ($(git rev-parse --short HEAD))."
		push_tag="refs/tags/$tag"
	else
		echo "release: $tag already exists on $(git rev-parse --short "$tagged"): no version-relevant change landed since that release, so nothing is tagged."
	fi
else
	git tag -a "$tag" -m "$tag"
	echo "release: tagged $tag on $(git rev-parse --short HEAD)."
	push_tag="refs/tags/$tag"
fi

# shellcheck disable=SC2086
git push "$remote" main $push_tag
