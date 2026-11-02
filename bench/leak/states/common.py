"""Common preparation for every state (bench/leak/BUILD.md § 2.2).

Run by the harness as `python3 -I states/common.py <tree>` before the state's own
script. Removes what an agent must not see or can't use, and nothing else:
the tracked agent configs of other tools, and the tracked 12 MB server binary
(it carries the old handler strings). `.claude/settings.json` and
`.claude/hooks/` stay: inject-agents.sh is how DeltaDocs puts AGENTS.md in a
Claude session. The state's own AGENTS.md stays.
"""
import os
import re
import shutil
import sys

REMOVE = [".claude/agents", ".cursor", ".pi", ".pi-lens.json", "backend/bin/server"]


def prepare(tree):
    for rel in REMOVE:
        p = os.path.join(tree, rel)
        if os.path.isdir(p):
            shutil.rmtree(p)
        elif os.path.exists(p):
            os.remove(p)
    manifest = os.path.join(tree, "cinch.yml")
    with open(manifest, encoding="utf-8") as f:
        text = f.read()
    # An exact pin (4c8af82's `cinch: 0.5.0+b`) becomes the floor pin every other
    # state has; cinch render then regenerates the workflows with the pinned cinch.
    fixed, n = re.subn(r"(?m)^(\s+cinch:)\s*(?!['\">])\S+\s*$", r"\1 '>=0.9.0'", text)
    if n:
        with open(manifest, "w", encoding="utf-8") as f:
            f.write(fixed)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: common.py <tree>")
    prepare(os.path.abspath(sys.argv[1]))
