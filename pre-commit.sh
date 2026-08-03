#!/usr/bin/env bash
# Install to .git/hooks/pre-commit (chmod +x).
# D024 — shell calls the binary and does nothing else.
set -euo pipefail
make check-harness
