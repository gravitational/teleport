#!/bin/bash

# this script is run once on initial install of the teleport upgrader package

set -eu

# reload systemd configuration and start/restart units
systemctl daemon-reload
# only reload teleport if the unit is already active, mimicking what systemctl try-restart does
# reloading when the unit is not already active (for example on a fresh install) generates an error
if systemctl is-active --quiet teleport.service; then
    systemctl reload teleport.service
fi
systemctl enable teleport-upgrade.timer
systemctl start teleport-upgrade.timer
