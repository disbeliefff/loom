#!/usr/bin/env bash
set -e

REPO="disbeliefff/loom"
BIN_NAME="loom"
INSTALL_DIR="/usr/local/bin"

echo "--- Installing $BIN_NAME ---"

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux) OS_NAME="Linux" ;;
    Darwin) OS_NAME="Darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH_NAME="x86_64" ;;
    arm64|aarch64) ARCH_NAME="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Check for curl
if ! command -v curl &> /dev/null; then
    echo "Error: curl is required to download the binary."
    exit 1
fi

# Get latest version
echo "Fetching latest version..."
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Failed to fetch the latest version. Are you rate-limited?"
    exit 1
fi

echo "Latest version is $VERSION"

# Construct download URL based on goreleaser template
DOWNLOAD_URL="https://github.com/disbeliefff/loom/releases/download/${VERSION}/${BIN_NAME}_${OS_NAME}_${ARCH_NAME}.tar.gz"

echo "Downloading from $DOWNLOAD_URL..."

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download and extract the binary
if ! curl -sL "$DOWNLOAD_URL" | tar -xz -C "$TMP_DIR" "$BIN_NAME"; then
    echo "Failed to download or extract the binary. Please check if the release exists."
    exit 1
fi

# Install the binary
echo "Installing to $INSTALL_DIR (might require sudo)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
else
    sudo mv "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
fi

chmod +x "$INSTALL_DIR/$BIN_NAME"

echo "✅ Successfully installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
echo "You can now use '$BIN_NAME' from your terminal."
