#!/bin/bash

# this script is run once on initial install of the teleport upgrader package

set -eu

# reload systemd configuration and start/restart units
systemctl daemon-reload
systemctl try-restart teleport.service
systemctl enable teleport-upgrade.timer
systemctl start teleport-upgrade.timer
