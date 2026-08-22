#!/bin/sh

set -euo pipefail

root_dir="$(git rev-parse --show-toplevel)"
version_file="$root_dir/internal/version/VERSION"

current="0.0.0"
if [ -f "$version_file" ]; then
	current="$(tr -d '[:space:]' <"$version_file")"
fi

if [[ ! "$current" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
	echo "update-version: could not parse VERSION file contents: \"$current\"" >&2
	exit 1
fi
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

type="${1:-}"
case "$type" in
show)
	printf '%s\n' "$current"
	exit 0
	;;
major)
	major=$((major + 1))
	minor=0
	patch=0
	;;
minor)
	minor=$((minor + 1))
	patch=0
	;;
patch)
	patch=$((patch + 1))
	;;
*)
	echo "Usage: scripts/update-version.sh <major|minor|patch|show>" >&2
	exit 1
	;;
esac

next="${major}.${minor}.${patch}"
printf '%s\n' "$next" >"$version_file"
echo "${current} -> ${next}"
