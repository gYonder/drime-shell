#!/bin/bash
set -e

OWNER="mikael-mansson"
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

TMP_DIR=$(mktemp -d)

echo "Downloading checksums..."
CHECKSUMS_URL="https://github.com/$OWNER/$REPO/releases/latest/download/checksums.txt"
curl -fsSL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt"

echo "Downloading $LATEST_URL..."
RELEASE_FILE="$TMP_DIR/release.$FORMAT"
curl -fsSL "$LATEST_URL" -o "$RELEASE_FILE"

echo "Verifying checksum..."
# Extract just the checksum for our file
EXPECTED_SUM=$(grep "$(basename "$LATEST_URL")" "$TMP_DIR/checksums.txt" | awk '{print $1}')

if [ -z "$EXPECTED_SUM" ]; then
  echo "Error: Could not find checksum for $(basename "$LATEST_URL")"
  exit 1
fi

# Calculate local checksum
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SUM=$(sha256sum "$RELEASE_FILE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_SUM=$(shasum -a 256 "$RELEASE_FILE" | awk '{print $1}')
else
  echo "Warning: sha256sum/shasum not found, skipping verification."
fi

if [ -n "$ACTUAL_SUM" ] && [ "$EXPECTED_SUM" != "$ACTUAL_SUM" ]; then
  echo "Error: Checksum verification failed!"
  echo "Expected: $EXPECTED_SUM"
  echo "Actual:   $ACTUAL_SUM"
  exit 1
fi
echo "Checksum verified: $ACTUAL_SUM"

echo "Extracting..."
tar -xzf "$TMP_DIR/release.$FORMAT" -C "$TMP_DIR"

echo "Installing to $BINDIR..."
mkdir -p "$BINDIR"

# Find the binary (handles whether it's in root or a subdir)
FOUND_BIN=$(find "$TMP_DIR" -type f -name "$BINARY" | head -n 1)

if [ -z "$FOUND_BIN" ]; then
  echo "Error: Binary '$BINARY' not found in downloaded release"
  exit 1
fi

mv "$FOUND_BIN" "$BINDIR/$BINARY"

chmod +x "$BINDIR/$BINARY"
rm -rf "$TMP_DIR"

echo "Successfully installed $BINARY to $BINDIR/$BINARY"

if ! echo ":$PATH:" | grep -q ":$BINDIR:"; then
  echo "Note: add $BINDIR to your PATH (e.g. in ~/.bashrc or ~/.zshrc):"
  echo "  export PATH=\"$BINDIR:\$PATH\""
fi
