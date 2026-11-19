#!/bin/bash
# harness-template v3
# SessionStart hook: print a quick orientation snapshot -- the current branch and the status of any
# active plan files -- so a session knows where it is without being told. AGENTS.md is not injected
# here: CLAUDE.md is `@AGENTS.md`, so Claude Code already loads it. The final confirmation line is
# the health signal that the hook ran: it is printed here by the hook (observable by the user), not
# emitted by the model, so it costs zero model tokens and can't be faked by a model that ignored the
# docs.

echo "=== Repo orientation (auto-injected) ==="
branch=$(git branch --show-current 2>/dev/null)
echo "branch: ${branch:-<detached or not a git repo>}"

# paths.docs now lives nested (paths: { docs: ... }) since cinch's manifest
# moved to YAML — a flat `paths.docs = ...` grep matches nothing there and
# silently falls through to .docs, which is exactly the "globs nothing,
# active plans: none forever" failure this is meant to prevent. Match docs:
# only within the paths: block, not anywhere else that key could appear.
docs_root=$(awk '
    /^[^[:space:]#]/ { in_paths = ($0 ~ /^paths:/) }
    in_paths && /^[[:space:]]+docs:[[:space:]]*/ {
        sub(/^[[:space:]]*docs:[[:space:]]*/, ""); sub(/[[:space:]]*(#.*)?$/, "")
        print; exit
    }
' cinch.yml 2>/dev/null)
docs_root="${docs_root:-.docs}"

# Plan-level status of any active plans. Scan only the top of each file so per-task
# '**Status:** [x]' checkboxes deeper in a decomposed plan aren't mistaken for the plan's own
# status. The fix/ subdir may not exist yet — the -e guard skips the unexpanded glob.
found_plan=false
for f in "$docs_root/plans"/*.md "$docs_root/plans/fix"/*.md; do
	[ -e "$f" ] || continue
	n=0
	while IFS= read -r line; do
		n=$((n + 1))
		[ "$n" -gt 15 ] && break
		case "$line" in
		Status:* | '**Status:**'* | '## Status'*)
			if [ "$found_plan" = false ]; then
				echo "active plans:"
				found_plan=true
			fi
			printf '  %s — %s\n' "${f#"$docs_root"/plans/}" "$line"
			break
			;;
		esac
	done <"$f"
done
[ "$found_plan" = false ] && echo "active plans: none with a status line"

echo
echo "[session-start hook ✓ repo orientation injected]"
