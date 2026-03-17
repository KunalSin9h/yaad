#!/usr/bin/env bash
set -euo pipefail

REPO="KunalSin9h/yaad"
BIN="yaad"

# ── detect OS ────────────────────────────────────────────────────────────────
case "$(uname -s)" in
  Linux*)   OS="linux"   ;;
  Darwin*)  OS="darwin"  ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

# ── detect arch ──────────────────────────────────────────────────────────────
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

# ── resolve install dir ───────────────────────────────────────────────────────
if [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
  INSTALL_DIR="$HOME/.local/bin"
else
  INSTALL_DIR="$HOME/bin"
  mkdir -p "$INSTALL_DIR"
fi

# ── fetch latest release tag ──────────────────────────────────────────────────
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Could not determine latest release tag." >&2
  exit 1
fi

echo "Installing ${BIN} ${TAG} (${OS}/${ARCH})..."

# ── build download URL ────────────────────────────────────────────────────────
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

ARCHIVE="${BIN}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

# ── download & extract ────────────────────────────────────────────────────────
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$ARCHIVE"

if [ "$EXT" = "zip" ]; then
  unzip -q "$TMP/$ARCHIVE" -d "$TMP"
else
  tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
fi

# ── install ───────────────────────────────────────────────────────────────────
BINARY_NAME="$BIN"
[ "$OS" = "windows" ] && BINARY_NAME="${BIN}.exe"

chmod +x "$TMP/$BINARY_NAME"
mv "$TMP/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo "${BIN} ${TAG} installed to ${INSTALL_DIR}/${BINARY_NAME}"

# ── PATH hint ─────────────────────────────────────────────────────────────────
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
  echo ""
  echo "Add ${INSTALL_DIR} to your PATH:"
  echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
fi
