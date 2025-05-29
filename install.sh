#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR=/usr/local/bin
SYSTEMD_DIR="${HOME}/.config/systemd/user"

check_sudo() {
    if [[ "$EUID" == 0 ]]; then
        echo "error: do not run as root (sudo will be used when needed)" >&2
        exit 1
    fi
}

# Run script
check_sudo

if systemctl --user is-active --quiet vltd; then
    echo "stopping running vltd service"
    systemctl --user stop vltd
fi

echo "installing binaries to $INSTALL_DIR"
sudo cp "${SCRIPT_DIR}"/{vlt,vltd} "$INSTALL_DIR"

echo "installing systemd unit to $SYSTEMD_DIR"
mkdir -p "$SYSTEMD_DIR"
cp "${SCRIPT_DIR}/systemd/vltd.service" "$SYSTEMD_DIR"

echo "reloading systemd user daemon"
systemctl --user daemon-reload

echo "enabling and starting vltd"
systemctl --user enable --now vltd

echo ok.
