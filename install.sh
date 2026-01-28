#!/bin/bash
# .vibe installer - because even installing should be effortless

set -e

REPO="codecravings/.vibe"
BINARY=".vibe"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case $OS in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    mingw*|msys*|cygwin*) OS="windows"; BINARY=".vibe.exe" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest release
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
    echo "Could not fetch latest release. Using go install instead..."
    go install github.com/$REPO@latest
    exit 0
fi

URL="https://github.com/$REPO/releases/download/$LATEST/.vibe-$OS-$ARCH"
[ "$OS" = "windows" ] && URL="$URL.exe"

echo "Downloading .vibe $LATEST for $OS/$ARCH..."
curl -sL "$URL" -o /tmp/$BINARY
chmod +x /tmp/$BINARY

# Install
if [ -w /usr/local/bin ]; then
    mv /tmp/$BINARY /usr/local/bin/.vibe
else
    sudo mv /tmp/$BINARY /usr/local/bin/.vibe
fi

echo "Installed .vibe to /usr/local/bin/.vibe"
echo "Run: .vibe --help"
