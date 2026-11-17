"""Extra anchored edits for the T3 state scripts (kept apart from anchors.py,
which the other tasks share)."""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a


def delete_md_section(text, heading):
    """Delete a markdown section: the line equal to `heading` through the line
    before the next heading of the same or a higher level (or end of file)."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.strip() == heading]
    if len(hits) != 1:
        a.die("heading %r matched %d times, want 1" % (heading, len(hits)))
    start = hits[0]
    level = len(heading) - len(heading.lstrip("#"))
    end = start + 1
    while end < len(lines):
        m = re.match(r"^(#+)\s", lines[end])
        if m and len(m.group(1)) <= level:
            break
        end += 1
    del lines[start:end]
    return "\n".join(lines)


_IMPORT_LINE = re.compile(r'^\t(?:(\w+|_|\.)\s+)?"([^"]+)"\s*$')


def prune_unused_imports(text):
    """Drop import lines of a single parenthesised import block whose package
    name no longer appears as `name.` in the rest of the file."""
    lines = text.split("\n")
    try:
        start = lines.index("import (")
        end = lines.index(")", start)
    except ValueError:
        return text
    body = "\n".join(lines[end + 1 :])
    keep = []
    for line in lines[start + 1 : end]:
        m = _IMPORT_LINE.match(line)
        if not m:
            keep.append(line)
            continue
        alias, path = m.group(1), m.group(2)
        if alias in ("_", "."):
            keep.append(line)
            continue
        name = alias or path.rsplit("/", 1)[-1]
        if re.search(r"\b%s\." % re.escape(name), body):
            keep.append(line)
    # collapse blank-line runs the removal may leave in the block
    out = []
    for line in keep:
        if line == "" and (not out or out[-1] == ""):
            continue
        out.append(line)
    while out and out[-1] == "":
        out.pop()
    return "\n".join(lines[: start + 1] + out + lines[end:])
