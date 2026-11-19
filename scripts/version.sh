#!/bin/sh
# harness-template v3
# scripts/version.sh <show|major|minor|patch|file>: the repo's version, kept in a root
# VERSION file holding `x.y.z` (one line). Managed by the harness template, never edited in a repo.
#
#   show    print x.y.z (the release tag is v<x.y.z>)
#   major   bump the major, reset minor and patch; rewrite VERSION and print `old -> new`
#   minor   bump the minor, reset patch; same
#   patch   bump the patch; same
#   file    print the repo-relative path this script owns (VERSION), for `git add`
#
# Works from any directory in the repo. VERSION_FILE overrides the file (for tests).
# Exit status: 0 ok, 1 missing or malformed file, 2 usage.
set -eu

usage() {
	echo "usage: scripts/version.sh <show|major|minor|patch|file>" >&2
	exit 2
}

[ "$#" -eq 1 ] || usage
cmd=$1
case "$cmd" in
show | major | minor | patch) ;;
file)
	echo VERSION
	exit 0
	;;
*) usage ;;
esac

file=${VERSION_FILE:-$(git rev-parse --show-toplevel)/VERSION}
if [ ! -f "$file" ]; then
	echo "version: $file not found (expected one line: x.y.z)" >&2
	exit 1
fi

current=$(head -n 1 "$file" | tr -d ' \t\r')
oldifs=$IFS
IFS=.
# shellcheck disable=SC2086
set -- $current
IFS=$oldifs
if [ "$#" -ne 3 ]; then
	echo "version: $file does not hold x.y.z (got '$current')" >&2
	exit 1
fi
major=$1
minor=$2
patch=$3
for n in "$major" "$minor" "$patch"; do
	case "$n" in
	'' | *[!0-9]* | 0?*)
		echo "version: $file does not hold x.y.z (got '$current')" >&2
		exit 1
		;;
	esac
done

case "$cmd" in
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
esac

next="$major.$minor.$patch"
printf '%s\n' "$next" >"$file"
echo "$current -> $next"
