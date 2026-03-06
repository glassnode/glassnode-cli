#!/usr/bin/env bash
set -e

REPO="glassnode/gn"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

case "$(uname -s)" in
  Darwin) GOOS=darwin ;;
  Linux)  GOOS=linux ;;
  *)
    echo "Unsupported OS: $(uname -s). Use Windows install script on Windows." >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64)  GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *)
    echo "Unsupported arch: $(uname -m)" >&2
    exit 1
    ;;
esac

VERSION=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
[ -z "$VERSION" ] && { echo "Could not resolve latest release." >&2; exit 1; }

ASSET="gn_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Installing gn v${VERSION} (${GOOS}/${GOARCH}) to ${INSTALL_DIR}..."
curl -sSfL -o "$TMP/checksums.txt" "$CHECKSUMS_URL"
curl -sSfL -o "$TMP/archive.tar.gz" "$URL"

# Verify checksum (format: "SHA256  filename" or "hash  filename")
if command -v sha256sum >/dev/null 2>&1; then
  SUM=$(sha256sum -b "$TMP/archive.tar.gz" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  SUM=$(shasum -a 256 -b "$TMP/archive.tar.gz" | awk '{print $1}')
else
  echo "Neither sha256sum nor shasum found. Install one to verify checksums." >&2
  exit 1
fi
EXPECTED=$(grep -F "$ASSET" "$TMP/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ] || [ "$SUM" != "$EXPECTED" ]; then
  echo "Checksum mismatch. Expected $EXPECTED, got $SUM" >&2
  exit 1
fi

tar -xzf "$TMP/archive.tar.gz" -C "$TMP"

if [ ! -w "$INSTALL_DIR" ]; then
  SUDO=sudo
else
  SUDO=
fi
$SUDO mkdir -p "$INSTALL_DIR"
chmod +x "$TMP/gn"
$SUDO mv "$TMP/gn" "$INSTALL_DIR/gn"
echo "Installed: ${INSTALL_DIR}/gn"
