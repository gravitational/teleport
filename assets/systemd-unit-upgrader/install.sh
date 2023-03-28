#!/bin/bash

set -euo pipefail

installer_kind="${1:-nop}"

log_info() {
    echo "[i] ${@} [ $(caller | awk '{print $1}') ]" >&2
}

# validate installer kind
case $installer_kind in
    apt | yum)
        log_info "upgrader will be configured to use $installer_kind installer."
        ;;
    nop)
        log_info "upgrader will be configured to use $installer_kind installer (testing purposes only)."
        ;;
    *)
        log_info "unsupported installer kind: '$installer_kind' (must be one of apt, yum, or nop)"
        exit 1
        ;;
esac

# install teleport-upgrade script
cp teleport-upgrade /usr/local/bin/

# install teleport-upgrade timer
cp teleport-upgrade.service /etc/systemd/system/
cp teleport-upgrade.timer /etc/systemd/system/

# set up initial unit configuration
mkdir -p /etc/teleport-upgrade.d
echo "$installer_kind" > /etc/teleport-upgrade.d/installer

# set up env var injection for teleport.service
mkdir -p /etc/systemd/system/teleport.service.d/
cp teleport-upgrader-env.conf /etc/systemd/system/teleport.service.d/

# reload systemd configuration and start/restart units
systemctl daemon-reload
systemctl try-restart teleport.service
systemctl start teleport-upgrade.timer
