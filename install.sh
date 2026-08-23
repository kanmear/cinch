#!/usr/bin/env bash
set -euo pipefail

REPO="kanmear/cinch"
BINARY="cinch"

err() { printf 'install: %s\n' "$1" >&2; exit 1; }
info() { printf 'install: %s\n' "$1"; }

VERSION="${CINCH_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "resolving latest release..."
  LATEST_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
  VERSION="$(printf '%s\n' "$LATEST_JSON" | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [ -n "$VERSION" ] || err "could not resolve latest version; set CINCH_VERSION=vX.Y.Z"
fi
VERSION="${VERSION#v}"
TAG="v${VERSION}"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) GOOS="darwin" ;;
  Linux) GOOS="linux" ;;
  *) err "unsupported OS: $OS (Windows users: download cinch_${VERSION}_windows_amd64.zip from https://github.com/${REPO}/releases/tag/${TAG})" ;;
esac

case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *) err "unsupported architecture: $ARCH" ;;
esac

ARCHIVE="cinch_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

info "downloading ${ARCHIVE} (${TAG})..."
curl -fsSL "${BASE_URL}/${ARCHIVE}" -o "${WORKDIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${WORKDIR}/checksums.txt"

(
  cd "$WORKDIR"
  if command -v sha256sum >/dev/null 2>&1; then
    grep " ${ARCHIVE}\$" checksums.txt | sha256sum -c - >/dev/null \
      || err "checksum verification failed for ${ARCHIVE}"
  elif command -v shasum >/dev/null 2>&1; then
    grep " ${ARCHIVE}\$" checksums.txt | shasum -a 256 -c - >/dev/null \
      || err "checksum verification failed for ${ARCHIVE}"
  else
    info "warning: no sha256sum/shasum found, skipping checksum verification"
  fi
)

tar -xzf "${WORKDIR}/${ARCHIVE}" -C "$WORKDIR"

install_dir="${CINCH_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
    mkdir -p "$install_dir"
  fi
fi

install -m 755 "${WORKDIR}/${BINARY}" "${install_dir}/${BINARY}" 2>/dev/null \
  || { cp "${WORKDIR}/${BINARY}" "${install_dir}/${BINARY}"; chmod 755 "${install_dir}/${BINARY}"; }

info "installed ${BINARY} ${VERSION} -> ${install_dir}/${BINARY}"

case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) info "note: ${install_dir} is not on your PATH. Add it, e.g.:
       export PATH=\"${install_dir}:\$PATH\"" ;;
esac

"${install_dir}/${BINARY}" version >/dev/null 2>&1 && info "done: $(${install_dir}/${BINARY} version)"
