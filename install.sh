#!/bin/sh
set -e

# Matcha installer — downloads the latest binary from GitHub releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/karloscodes/matcha/main/install.sh | sh

REPO="karloscodes/matcha"
INSTALL_DIR="/usr/local/bin"
BINARY="matcha"

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
  echo "Error: matcha only runs on Linux (got $OS)" >&2
  exit 1
fi

ASSET="${BINARY}-${OS}-${ARCH}"

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)

if [ -z "$TAG" ]; then
  echo "Error: could not determine latest release" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "Downloading matcha ${TAG} (${OS}/${ARCH})..."
curl -fsSL "$URL" -o "/tmp/${BINARY}"
chmod +x "/tmp/${BINARY}"
mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Installed matcha ${TAG} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  matcha setup              # Set up Docker, network, proxy"
echo "  matcha add <name> ...     # Register an app"
echo "  matcha deploy <name>      # Deploy it"
