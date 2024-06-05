#!/bin/bash

# this script is run each time the teleport upgrader package is itself upgraded

set -eu

# skip reload and restart when systemd is disabled. This is only relevant when
# testing in a container.
if [ -d "/run/systemd/system" ]; then
    # reload systemd configuration and restart timer unit
    systemctl daemon-reload
    systemctl try-restart teleport-upgrade.timer
fi
