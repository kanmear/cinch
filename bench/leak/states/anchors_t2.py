"""Helpers for the T2 state scripts that anchors.py does not have."""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import anchors as a


def extract_go_func(text, header):
    """The Go function whose `func ...` line starts with `header`, with the
    comment lines directly above it, as it stands in `text` (no trailing newline)."""
    begin, stop = a.go_func_bounds(text, header)
    return text[begin : stop + 1]


def append_go(text, block):
    """Append a Go block to a file, separated by one blank line."""
    return text.rstrip("\n") + "\n\n" + block.strip("\n") + "\n"
