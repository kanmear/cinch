"""Anchored edits for the state scripts (bench/leak/BUILD.md § 2).

Every edit names its anchor and asserts it matches exactly once, so a wrong base
fails loudly instead of silently editing nothing. Scripts run as
`python3 -I states/<task>-arm<k>.py <tree>` and import this module after putting
their own directory on sys.path.
"""
import os
import re
import subprocess
import sys


def die(msg):
    sys.stderr.write("state script: " + msg + "\n")
    sys.exit(1)


def env(name):
    value = os.environ.get(name, "")
    if not value:
        die("missing environment variable " + name)
    return value


def tree_arg():
    if len(sys.argv) != 2:
        die("usage: <script> <tree>")
    tree = os.path.abspath(sys.argv[1])
    if not os.path.isdir(tree):
        die(tree + " is not a directory")
    return tree


def path(tree, rel):
    return os.path.join(tree, rel)


def read(tree, rel):
    with open(path(tree, rel), encoding="utf-8") as f:
        return f.read()


def write(tree, rel, text):
    with open(path(tree, rel), "w", encoding="utf-8") as f:
        f.write(text)


def remove(tree, rel):
    """Delete a file or directory; it must exist."""
    p = path(tree, rel)
    if not os.path.exists(p):
        die("cannot remove " + rel + ": absent")
    if os.path.isdir(p):
        import shutil
        shutil.rmtree(p)
    else:
        os.remove(p)


def show(sha, rel):
    """A file as it was at a commit of the fixture, read-only (git show)."""
    out = subprocess.run(
        ["git", "-C", env("DELTADOCS"), "show", sha + ":" + rel],
        capture_output=True,
    )
    if out.returncode != 0:
        die("git show %s:%s failed: %s" % (sha, rel, out.stderr.decode()))
    return out.stdout.decode("utf-8")


def replace_once(text, old, new, what=""):
    n = text.count(old)
    if n != 1:
        die("anchor %s matched %d times, want 1: %r" % (what or "", n, old[:120]))
    return text.replace(old, new)


def delete_line(text, line, what=""):
    """Delete the one line that equals `line` (surrounding whitespace ignored)."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.strip() == line.strip()]
    if len(hits) != 1:
        die("line anchor %s matched %d times, want 1: %r" % (what or "", len(hits), line[:120]))
    del lines[hits[0]]
    return "\n".join(lines)


def _block_end(lines, start):
    """Index of the closing `}` at column 0 at or after `start`."""
    for i in range(start, len(lines)):
        if lines[i] == "}":
            return i
    die("no closing brace after line %d" % (start + 1))


def delete_go_func(text, header):
    """Delete the Go function whose `func ...` line starts with `header`,
    with the comment lines directly above it, through its closing `}` at column
    0 and one following blank line."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.startswith(header)]
    if len(hits) != 1:
        die("func anchor matched %d times, want 1: %r" % (len(hits), header))
    start = hits[0]
    end = _block_end(lines, start)
    first = start
    while first > 0 and lines[first - 1].startswith("//"):
        first -= 1
    last = end + 1
    if last < len(lines) and lines[last] == "":
        last += 1
    del lines[first:last]
    return "\n".join(lines)


def go_func_bounds(text, header):
    """(start, end) character offsets of a Go func, comments above included, for
    callers that edit inside it."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.startswith(header)]
    if len(hits) != 1:
        die("func anchor matched %d times, want 1: %r" % (len(hits), header))
    start = hits[0]
    end = _block_end(lines, start)
    first = start
    while first > 0 and lines[first - 1].startswith("//"):
        first -= 1
    begin = sum(len(l) + 1 for l in lines[:first])
    stop = sum(len(l) + 1 for l in lines[: end + 1])
    return begin, stop - 1


def edit_in_go_func(text, header, fn):
    """Apply fn(body) to the text of one Go function and splice it back."""
    begin, stop = go_func_bounds(text, header)
    return text[:begin] + fn(text[begin:stop]) + text[stop:]


def delete_test(text, name):
    """Delete `func <name>(` with the comments directly above it (rule markers
    included)."""
    return delete_go_func(text, "func " + name + "(")


def marker_comment(text, test_name):
    """The comment lines directly above `func <test_name>(`."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.startswith("func " + test_name + "(")]
    if len(hits) != 1:
        die("test anchor %s matched %d times, want 1" % (test_name, len(hits)))
    first = hits[0]
    while first > 0 and lines[first - 1].startswith("//"):
        first -= 1
    return lines[first : hits[0]]


def set_marker_comment(text, test_name, comment_lines):
    """Replace the comment block directly above `func <test_name>(`."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if l.startswith("func " + test_name + "(")]
    if len(hits) != 1:
        die("test anchor %s matched %d times, want 1" % (test_name, len(hits)))
    first = hits[0]
    while first > 0 and lines[first - 1].startswith("//"):
        first -= 1
    lines[first : hits[0]] = comment_lines
    return "\n".join(lines)


_ITEM = re.compile(r"^\d+\.\s+\*\*([A-Z]+-\d+)\*\*")


def delete_rule_item(text, rule_id):
    """Delete a numbered list item `N. **ID** ...`, with its indented
    continuation lines (a following `<!-- cinch:ignore ... -->` included)."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if (_ITEM.match(l) or [None, None])[1] == rule_id]
    if len(hits) != 1:
        die("rule item %s matched %d times, want 1" % (rule_id, len(hits)))
    start = hits[0]
    end = start + 1
    while end < len(lines) and lines[end].startswith((" ", "\t")) and lines[end].strip():
        end += 1
    del lines[start:end]
    return "\n".join(lines)


def rule_item(text, rule_id):
    """The text of a rule's list item with its continuation lines."""
    lines = text.split("\n")
    hits = [i for i, l in enumerate(lines) if (_ITEM.match(l) or [None, None])[1] == rule_id]
    if len(hits) != 1:
        die("rule item %s matched %d times, want 1" % (rule_id, len(hits)))
    start = hits[0]
    end = start + 1
    while end < len(lines) and lines[end].startswith((" ", "\t")) and lines[end].strip():
        end += 1
    return "\n".join(lines[start:end])


def no_match(tree, pattern, subdir="backend"):
    """Post-condition: `grep -rEn pattern subdir` finds nothing."""
    out = subprocess.run(["grep", "-rEn", pattern, subdir], cwd=tree, capture_output=True)
    if out.returncode == 0:
        die("post-condition: %r still matches under %s:\n%s" % (pattern, subdir, out.stdout.decode()[:2000]))
