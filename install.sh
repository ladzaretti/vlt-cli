#!/bin/bash
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    echo "Usage: $0 [version]"
    echo
    echo "If no version is provided, the latest release will be installed."
    echo
    echo "Examples:"
    echo "  $0           # installs the latest version"
    echo "  $0 0.2.0     # installs version 0.2.0"
    exit 0
fi

OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
x86_64) ARCH="amd64" ;;
aarch64 | arm64) ARCH="arm64" ;;
i386 | i686) ARCH="386" ;;
*)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

VERSION=${1:-latest}
TMP_DIR=$(mktemp -d)

trap 'rm -rf "$TMP_DIR"' EXIT

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -s https://api.github.com/repos/ladzaretti/vlt-cli/releases/latest |
        grep '"tag_name":' |
        sed -E 's/.*"v([^"]+)".*/\1/')
fi

if [[ -z "$VERSION" ]]; then
    echo "Failed to determine version" >&2
    exit 1
fi

echo "Preparing install:"
echo "  OS      : $OS"
echo "  ARCH    : $ARCH"
echo "  VERSION : $VERSION"

TARBALL=vlt_${VERSION}_${OS}_${ARCH}.tar.gz
DEST="$TMP_DIR"/"$TARBALL"
URL="https://github.com/ladzaretti/vlt-cli/releases/download/v$VERSION/$TARBALL"

echo "Downloading $URL..."
curl -sSL --fail-with-body -o "$DEST" "$URL"

echo "OK."

echo "Extracting archive..."
tar -xz -C "$TMP_DIR" -f "$DEST"

echo "OK."

cd "$TMP_DIR/vlt_${VERSION}_${OS}_${ARCH}"

echo "Running installer..."

if [[ ! -x install.sh ]]; then
    echo "install.sh not found or not executable in $PWD" >&2
    exit 1
fi

./install.sh
