#!/usr/bin/env bash
set -Eeuo pipefail

VERSION="${1:-latest}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL https://api.github.com/repos/dakaneye/release-pilot/releases/latest | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
fi

URL="https://github.com/dakaneye/release-pilot/releases/download/${VERSION}/release-pilot_${VERSION#v}_${OS}_${ARCH}.tar.gz"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "$TMPDIR/release-pilot.tar.gz"
tar -xzf "$TMPDIR/release-pilot.tar.gz" -C "$TMPDIR"
install -m 755 "$TMPDIR/release-pilot" /usr/local/bin/release-pilot

echo "release-pilot ${VERSION} installed"
