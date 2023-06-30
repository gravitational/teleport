#!/bin/bash

# this script is run each time the teleport upgrader package is itself upgraded

set -eu

# reload systemd configuration and restart timer unit
systemctl daemon-reload
systemctl try-restart teleport-upgrade.timer
