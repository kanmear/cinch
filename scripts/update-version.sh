#!/bin/sh

set -eu

root_dir="$(git rev-parse --show-toplevel)"
version_dir="$root_dir/internal/version"

alphabet="abcdefghijklmnopqrstuvwxyz"

read_field() {
	f="$version_dir/$1"
	if [ -f "$f" ]; then
		tr -d '[:space:]' <"$f"
	else
		printf '0'
	fi
}

major="$(read_field major)"
minor="$(read_field minor)"
patch="$(read_field patch)"
hotfix="$(read_field hotfix)"

for field_val in "$major" "$minor" "$patch" "$hotfix"; do
	case "$field_val" in
	'' | *[!0-9]*)
		echo "update-version: could not parse version fields in $version_dir" >&2
		exit 1
		;;
	esac
done

# hotfix_letter is bijective base-26: 0 -> (empty), 1 -> a, 26 -> z, 27 -> aa.
hotfix_letter() {
	n="$1"
	s=""
	while [ "$n" -gt 0 ]; do
		n=$((n - 1))
		r=$((n % 26 + 1))
		c=$(printf '%s' "$alphabet" | cut -c"$r")
		s="${c}${s}"
		n=$((n / 26))
	done
	printf '%s' "$s"
}

format() {
	printf '%s.%s.%s%s' "$1" "$2" "$3" "$(hotfix_letter "$4")"
}

current="$(format "$major" "$minor" "$patch" "$hotfix")"

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
	hotfix=0
	;;
minor)
	minor=$((minor + 1))
	patch=0
	hotfix=0
	;;
patch)
	patch=$((patch + 1))
	hotfix=0
	;;
hotfix)
	hotfix=$((hotfix + 1))
	;;
*)
	echo "Usage: scripts/update-version.sh <major|minor|patch|hotfix|show>" >&2
	exit 1
	;;
esac

next="$(format "$major" "$minor" "$patch" "$hotfix")"
printf '%s\n' "$major" >"$version_dir/major"
printf '%s\n' "$minor" >"$version_dir/minor"
printf '%s\n' "$patch" >"$version_dir/patch"
printf '%s\n' "$hotfix" >"$version_dir/hotfix"
echo "${current} -> ${next}"
