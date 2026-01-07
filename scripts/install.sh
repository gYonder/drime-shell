#!/bin/bash
set -euo pipefail

# Require bash 3.0+ for BASH_REMATCH
[[ -n "${BASH_VERSION:-}" ]] || { echo "error: bash required (not sh/dash/ash)" >&2; exit 1; }
[[ "${BASH_VERSINFO[0]}" -ge 3 ]] || { echo "error: bash 3.0+ required (found $BASH_VERSION)" >&2; exit 1; }

REPO="drime-shell"
BINARY="drime"
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
        echo "  curl -fsSL https://raw.githubusercontent.com/mikael-mansson/drime-shell/main/scripts/install.sh | bash"
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
command -v mktemp >/dev/null || error "mktemp is required"
command -v awk >/dev/null || error "awk is required"
command -v grep >/dev/null || error "grep is required"
(command -v shasum >/dev/null || command -v sha256sum >/dev/null) || error "shasum or sha256sum is required"

banner

# Resolve latest release tag (via /releases/latest redirect, not asset URL which goes to CDN)
info "Checking latest version..."
LATEST_URL=$(curl -fsSI --connect-timeout 10 --max-time 30 "https://github.com/mikael-mansson/${REPO}/releases/latest" 2>/dev/null | grep -i '^location:' | tr -d '\r' | awk '{print $2}')
[[ "$LATEST_URL" =~ /tag/([^/]+)$ ]] || error "could not determine latest version (check network or GitHub status)"
TAG="${BASH_REMATCH[1]}"
VERSION="${TAG#v}"

FILENAME="${REPO}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/mikael-mansson/${REPO}/releases/download/${TAG}/${FILENAME}"

# Skip if current (works even if PATH isn't updated yet)
CURRENT=$({ "$INSTALL_DIR/$BINARY" --version 2>/dev/null || "$BINARY" --version 2>/dev/null; } || true)
CURRENT=${CURRENT//$'\r'/}
CURRENT=${CURRENT//$'\n'/}
# Handle verbose format: "drime-shell version X.Y.Z (commit...)" or "version vX.Y.Z"
[[ "$CURRENT" =~ version[[:space:]]+v?([0-9]+\.[0-9]+\.[0-9]+[^[:space:]]*) ]] && CURRENT="${BASH_REMATCH[1]}"
CURRENT=${CURRENT#v}
CURRENT=${CURRENT%% *}  # Trim any trailing content
[[ "$CURRENT" == "$VERSION" ]] && { success "Already up to date ($VERSION)"; exit 0; }

info "Downloading $TAG..."
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'drime')  # BusyBox fallback
curl -fsSL --connect-timeout 10 --max-time 300 "$DOWNLOAD_URL" -o "$TMP_DIR/archive.tar.gz" || error "download failed (URL: $DOWNLOAD_URL)"

# Verify checksum
CHECKSUM_URL="https://github.com/mikael-mansson/${REPO}/releases/download/${TAG}/${REPO}_${VERSION}_checksums.txt"
EXPECTED=$(curl -fsSL --connect-timeout 10 --max-time 30 "$CHECKSUM_URL" | awk -v f="$FILENAME" '$2==f || $2==("./" f) {print $1; exit}')
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
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    SHELL_NAME=$(basename "${SHELL:-sh}")
    case "$SHELL_NAME" in
        zsh)  RC="${ZDOTDIR:-$HOME}/.zshrc" ;;
        bash) RC="$HOME/.bashrc" ;;
        fish) RC="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
        *)    RC="$HOME/.profile" ;;
    esac
    mkdir -p "$(dirname "$RC")"
    if ! grep -Fq "$INSTALL_DIR" "$RC" 2>/dev/null; then
        [[ "$SHELL_NAME" == "fish" ]] && echo "fish_add_path \"$INSTALL_DIR\"" >> "$RC" || echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$RC"
    fi
fi

echo
[[ -n "$CURRENT" ]] && success "Updated: $CURRENT → $VERSION" || success "Installed: $VERSION"
info "Restart your terminal, then run: drime"
