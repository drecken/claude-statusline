#!/usr/bin/env bash
# claude-statusline installer.
#
# Downloads pre-built binaries from GitHub Releases, installs them into
# ~/.claude/bin/, and prints the settings.json snippet to wire them up.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/drecken/claude-statusline/main/install.sh | bash
#   ./install.sh                           # install latest
#   VERSION=v0.1.0 ./install.sh            # install specific version
#   INSTALL_DIR=/somewhere ./install.sh    # override install location

set -euo pipefail

REPO="drecken/claude-statusline"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.claude/bin}"
VERSION="${VERSION:-latest}"

err() { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }
info() { printf '\033[36m==>\033[0m %s\n' "$*"; }
ok() { printf '\033[32mok:\033[0m %s\n' "$*"; }

need() { command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"; }
need curl
need tar
need uname

detect_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux)  echo linux ;;
    *)      err "unsupported OS: $(uname -s) (only darwin and linux are supported)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) err "unsupported arch: $(uname -m) (only amd64 and arm64 are supported)" ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$VERSION" = "latest" ]; then
  info "resolving latest release for $REPO"
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' \
    | head -n1 \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [ -n "$VERSION" ] || err "could not resolve latest release tag"
fi

ARCHIVE="claude-statusline_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
TARBALL_URL="$BASE_URL/$ARCHIVE"
CHECKSUMS_URL="$BASE_URL/checksums.txt"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

info "downloading $VERSION ($OS/$ARCH)"
curl -fsSL --retry 3 -o "$TMPDIR/$ARCHIVE" "$TARBALL_URL" \
  || err "download failed: $TARBALL_URL"
curl -fsSL --retry 3 -o "$TMPDIR/checksums.txt" "$CHECKSUMS_URL" \
  || err "download failed: $CHECKSUMS_URL"

info "verifying checksum"
EXPECTED="$(grep " $ARCHIVE\$" "$TMPDIR/checksums.txt" | awk '{print $1}')"
[ -n "$EXPECTED" ] || err "no checksum entry for $ARCHIVE in checksums.txt"

if command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$TMPDIR/$ARCHIVE" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMPDIR/$ARCHIVE" | awk '{print $1}')"
else
  err "need shasum or sha256sum to verify download"
fi

[ "$EXPECTED" = "$ACTUAL" ] \
  || err "checksum mismatch: expected $EXPECTED, got $ACTUAL"

info "extracting"
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMPDIR/statusline" "$INSTALL_DIR/statusline"
install -m 0755 "$TMPDIR/subagent-statusline" "$INSTALL_DIR/subagent-statusline"

# On macOS, strip the quarantine xattr so Gatekeeper doesn't block the binaries
# when they were downloaded by curl. No-op on Linux or if xattr is not set.
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/statusline" 2>/dev/null || true
  xattr -d com.apple.quarantine "$INSTALL_DIR/subagent-statusline" 2>/dev/null || true
fi

ok "installed $VERSION to $INSTALL_DIR"

cat <<EOF

Add these keys to ~/.claude/settings.json:

  "statusLine": {
    "type": "command",
    "command": "$INSTALL_DIR/statusline",
    "padding": 0,
    "refreshInterval": 30
  },
  "subagentStatusLine": {
    "type": "command",
    "command": "$INSTALL_DIR/subagent-statusline"
  }

Verify the binary:
  echo '{"workspace":{"current_dir":"'"\$PWD"'"},"session_id":"test","model":{"display_name":"Opus 4.7"},"context_window":{"used_percentage":42}}' | $INSTALL_DIR/statusline
EOF
