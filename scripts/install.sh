#!/bin/bash
set -e

OWNER="mikael.mansson2"
REPO="drime-shell"
BINARY="drime"
FORMAT="tar.gz"
BINDIR="${BINDIR:-$HOME/.local/bin}"

OS=$(uname -s)
ARCH=$(uname -m)

case $OS in
  Linux) OS="Linux" ;;
  Darwin) OS="Darwin" ;;
  *) echo "OS $OS not supported"; exit 1 ;;
esac

case $ARCH in
  x86_64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Architecture $ARCH not supported"; exit 1 ;;
esac

echo "Finding latest release..."
LATEST_URL="https://github.com/$OWNER/$REPO/releases/latest/download/${REPO}_${OS}_${ARCH}.${FORMAT}"

echo "Downloading $LATEST_URL..."
TMP_DIR=$(mktemp -d)
curl -fsSL "$LATEST_URL" -o "$TMP_DIR/release.$FORMAT"

echo "Extracting..."
tar -xzf "$TMP_DIR/release.$FORMAT" -C "$TMP_DIR"

echo "Installing to $BINDIR..."
mkdir -p "$BINDIR"
mv "$TMP_DIR/$BINARY" "$BINDIR/$BINARY"

chmod +x "$BINDIR/$BINARY"
rm -rf "$TMP_DIR"

echo "Successfully installed $BINARY to $BINDIR/$BINARY"

if ! echo ":$PATH:" | grep -q ":$BINDIR:"; then
  echo "Note: add $BINDIR to your PATH (e.g. in ~/.bashrc or ~/.zshrc):"
  echo "  export PATH=\"$BINDIR:\$PATH\""
fi
