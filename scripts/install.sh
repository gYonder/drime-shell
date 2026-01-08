#!/bin/bash
set -euo pipefail

REPO="drime-shell"
BINARY="drime-shell"
INSTALL_DIR="$HOME/.drime-shell/bin"

# Colors (disabled if not tty)
[[ -t 1 ]] && { RED='\033[31m'; GREEN='\033[32m'; DIM='\033[2m'; NC='\033[0m'; } || { RED=''; GREEN=''; DIM=''; NC=''; }
info()    { echo -e "${DIM}$*${NC}"; }
success() { echo -e "${GREEN}$*${NC}"; }
error()   { echo -e "${RED}error: $*${NC}" >&2; exit 1; }

banner() {
    echo -e "${GREEN}"
    echo "  ___      _              ___ _        _ _ "
    echo " |   \\ _ _(_)_ __  ___   / __| |_  ___| | |"
    echo " | |) | '_| | '  \\/ -_)  \\__ \\ ' \\/ -_) | |"
    echo " |___/|_| |_|_|_|_\\___|  |___/_||_\\___|_|_|"
    echo -e "${NC}"
}

TMP_DIR=""
cleanup() { [[ -d "${TMP_DIR:-}" ]] && rm -rf "$TMP_DIR"; return 0; }
trap cleanup EXIT

case "${1:-}" in
    -h|--help)
        echo "Drime Shell Installer"
        echo "Usage: install.sh [--uninstall]"
        echo "  curl -fsSL https://raw.githubusercontent.com/gYonder/drime-shell/main/scripts/install.sh | bash"
        exit 0 ;;
    -u|--uninstall)
        info "Uninstalling Drime Shell..."
        BIN="${INSTALL_DIR}/${BINARY}"
        [[ -f "$BIN" ]] || BIN=$(command -v "$BINARY" 2>/dev/null || true)
        if [[ -f "$BIN" ]]; then
            [[ -w "$BIN" ]] || error "not found or not writable"
            rm "$BIN"
            success "Removed $BIN"
        fi
        exit 0 ;;
    "") ;; # install
    *) error "unknown option: $1" ;;
esac

# Detect OS/Arch
OS=$(uname -s)
ARCH=$(uname -m)
case "$OS" in Darwin|Linux) ;; *) error "unsupported OS: $OS" ;; esac
case "$ARCH" in x86_64) ;; aarch64|arm64) ARCH="arm64" ;; *) error "unsupported arch: $ARCH" ;; esac
[[ "$OS" == "Darwin" && "$ARCH" == "x86_64" ]] && sysctl -n sysctl.proc_translated 2>/dev/null | grep -q 1 && ARCH="arm64"

command -v curl >/dev/null || error "curl is required"
command -v tar >/dev/null || error "tar is required"
(command -v shasum >/dev/null || command -v sha256sum >/dev/null) || error "shasum or sha256sum is required"

banner

# Resolve latest release tag (GitHub API returns releases in reverse chronological order)
info "Checking latest version..."
TAG=$(curl -fsSL --connect-timeout 10 --max-time 30 "https://api.github.com/repos/mikael-mansson/${REPO}/releases" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4)
if [[ -z "$TAG" ]]; then
    error "could not determine latest version (check network or GitHub status)"
fi
VERSION="${TAG#v}"

FILENAME="${REPO}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/mikael-mansson/${REPO}/releases/download/${TAG}/${FILENAME}"

# Skip if current (works even if PATH isn't updated yet)
CURRENT=$({ "$INSTALL_DIR/$BINARY" --version 2>/dev/null || "$BINARY" --version 2>/dev/null; } || true)
# Extract version number (works with BusyBox grep)
CURRENT=$(echo "$CURRENT" | tr ' ' '\n' | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+' | head -1 | sed 's/^v//' || true)
if [[ "$CURRENT" == "$VERSION" ]]; then
    success "Already up to date ($VERSION)"
    exit 0
fi

info "Downloading $TAG..."
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'drime')  # BusyBox fallback
curl -fsSL --connect-timeout 10 --max-time 300 "$DOWNLOAD_URL" -o "$TMP_DIR/archive.tar.gz" || error "download failed (URL: $DOWNLOAD_URL)"

# Verify checksum
CHECKSUM_URL="https://github.com/mikael-mansson/${REPO}/releases/download/${TAG}/${REPO}_${VERSION}_checksums.txt"
EXPECTED=$(curl -fsSL --connect-timeout 10 --max-time 30 "$CHECKSUM_URL" | grep "$FILENAME" | cut -d' ' -f1)
[[ -z "$EXPECTED" ]] && error "checksum not found for $FILENAME"
ACTUAL=$(shasum -a 256 "$TMP_DIR/archive.tar.gz" 2>/dev/null || sha256sum "$TMP_DIR/archive.tar.gz")
ACTUAL="${ACTUAL%% *}"
[[ "$EXPECTED" == "$ACTUAL" ]] || error "checksum mismatch"

# Install
info "Installing..."
tar -xzf "$TMP_DIR/archive.tar.gz" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/$BINARY"

# PATH setup
[[ "${GITHUB_ACTIONS:-}" == "true" && -n "${GITHUB_PATH:-}" ]] && echo "$INSTALL_DIR" >> "$GITHUB_PATH"
SHELL_NAME=$(basename "${SHELL:-sh}")
case "$SHELL_NAME" in
    zsh)  RC="${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash) RC="$HOME/.bashrc" ;;
    fish) RC="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
    *)    RC="$HOME/.profile" ;;
esac
PATH_ADDED=""
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    mkdir -p "$(dirname "$RC")"
    if ! grep -Fq "$INSTALL_DIR" "$RC" 2>/dev/null; then
        [[ "$SHELL_NAME" == "fish" ]] && echo "fish_add_path \"$INSTALL_DIR\"" >> "$RC" || echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$RC"
        PATH_ADDED="$RC"
    fi
fi

echo
[[ -n "$CURRENT" ]] && success "Updated: $CURRENT → $VERSION" || success "Installed: $VERSION"
if [[ -n "$PATH_ADDED" ]]; then
    info "Added to $PATH_ADDED"
fi
# Always show how to run if not in current PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    info "Run: source $RC && drime-shell"
else
    info "Run: drime-shell"
fi
