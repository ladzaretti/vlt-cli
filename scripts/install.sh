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

check_bin() {
    echo "validating binaries..."

    local ok=true

    for bin in vlt vltd; do
        if [[ ! -x "$SCRIPT_DIR/$bin" ]]; then
            echo "error: $bin not found or not executable at $SCRIPT_DIR" >&2
            ok=false
        fi

        if [[ -L "$INSTALL_DIR/$bin" ]]; then
            echo "error: $INSTALL_DIR/$bin is a symlink; refusing to overwrite" >&2
            ok=false
        fi
    done

    if [[ $ok == false ]]; then
        exit 1
    fi

    echo "OK."
}

check_systemd() {
    if ! command -v systemctl &>/dev/null; then
        echo "error: systemctl not found in PATH" >&2
        exit 1
    fi

    if ! systemctl --user show-environment &>/dev/null; then
        echo "error: systemctl --user is not available or not supported in this environment" >&2
        exit 1
    fi
}

# Run script
check_sudo
check_bin

echo "installing binaries to $INSTALL_DIR"
sudo -k cp "${SCRIPT_DIR}"/{vlt,vltd} "$INSTALL_DIR"

echo "OK."

read -rp "Install the vltd daemon systemd unit? [y/N] " answer

case "$answer" in
[Yy]*) ;;
*)
    echo "Skipping systemd unit installation."
    exit 0
    ;;
esac

check_systemd

if systemctl --user is-active --quiet vltd; then
    echo "stopping running vltd service"
    systemctl --user stop vltd
fi

echo "installing systemd unit to $SYSTEMD_DIR"
mkdir -p "$SYSTEMD_DIR"
cp "${SCRIPT_DIR}/systemd/vltd.service" "$SYSTEMD_DIR"

echo "reloading systemd user daemon"
systemctl --user daemon-reload

echo "enabling and starting vltd"
systemctl --user enable --now vltd

echo "OK."
