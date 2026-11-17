#!/bin/sh
# harness-template v2
# Runs one step of a make target quietly: the command's output goes to a log
# file and the terminal gets a single ✓/✗ line with the elapsed time.
#
#   scripts/step.sh "<label>" <cmd> [args...]
#
# On success: `  ✓ <label> (Ns)`. On failure: `  ✗ <label> (Ns)`, the last 30
# lines of the log and the log's path; the exit status is the command's.
# Logs live in `$(git rev-parse --git-dir)/make-logs/` (newest 50 kept).
#
# Environment:
#   V=1              stream the output (and keep it in the log) instead of
#                    hiding it; the ✓/✗ line is still printed.
#   STEP_DETAIL=last also print the command's last output line on success
#                    (e.g. the summary line of a backup command). Any other
#                    value is a sh snippet run as `sh -c "$STEP_DETAIL" _ <log>`
#                    whose output is printed (indented) on success, to pull a
#                    summary out of the log.
#   STEP_ANNOUNCE=1  print `  … <label>` before starting, for steps that take
#                    long enough that a silent terminal would look hung. Any
#                    other value is printed instead of the label.
#
# Plain POSIX sh with no dependencies.
set -u

if [ "$#" -lt 2 ]; then
	echo "usage: step.sh \"<label>\" <cmd> [args...]" >&2
	exit 2
fi

label=$1
shift

keep=50
tail_lines=30

log_dir="$(git rev-parse --absolute-git-dir 2>/dev/null || echo "${TMPDIR:-/tmp}")/make-logs"
mkdir -p "$log_dir"
slug=$(printf '%s' "$label" | tr 'A-Z' 'a-z' | tr -c 'a-z0-9' '-' | sed 's/--*/-/g; s/^-//; s/-$//' | cut -c1-40)
log="$log_dir/$(date +%Y%m%d-%H%M%S)-$$-${slug:-step}.log"

# Prune to the newest $keep logs (names are slugs, so no quoting trouble).
ls -1t "$log_dir" | tail -n +$((keep + 1)) | while read -r old; do
	rm -f "$log_dir/$old"
done

fmt_secs() {
	if [ "$1" -ge 60 ]; then
		printf '%dm%ds' $(($1 / 60)) $(($1 % 60))
	else
		printf '%ds' "$1"
	fi
}

if [ -n "${STEP_ANNOUNCE:-}" ] && [ "${V:-}" != 1 ]; then
	announce=$STEP_ANNOUNCE
	[ "$announce" = 1 ] && announce=$label
	echo "  … $announce"
fi

start=$(date +%s)
if [ "${V:-}" = 1 ]; then
	# No pipefail in POSIX sh: carry the command's status out through a file.
	status_file="$log.status"
	{
		"$@"
		echo $? >"$status_file"
	} 2>&1 | tee "$log"
	rc=$(cat "$status_file" 2>/dev/null || echo 1)
	rm -f "$status_file"
else
	"$@" >"$log" 2>&1
	rc=$?
fi
elapsed=$(($(date +%s) - start))

if [ "$rc" -eq 0 ]; then
	echo "  ✓ $label ($(fmt_secs "$elapsed"))"
	case "${STEP_DETAIL:-}" in
		"") ;;
		last)
			detail=$(tr -d '\r' <"$log" | grep -v '^[[:space:]]*$' | tail -n 1)
			[ -z "$detail" ] || echo "    $detail"
			;;
		*) sh -c "$STEP_DETAIL" _ "$log" | sed 's/^/    /' ;;
	esac
	exit 0
fi

echo "  ✗ $label ($(fmt_secs "$elapsed"))"
if [ "${V:-}" != 1 ]; then
	tail -n "$tail_lines" "$log" | sed 's/^/    /'
fi
echo "    full log: $log"
exit "$rc"
