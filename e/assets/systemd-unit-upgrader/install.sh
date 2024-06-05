#!/bin/bash

# this script is run once on initial install of the teleport upgrader package

set -eu

# skip reload and restart when systemd is disabled. This is only relevant when
# testing in a container.
if [ -d "/run/systemd/system" ]; then
    # reload systemd configuration and start/restart units
    systemctl daemon-reload

    # we should restart Teleport here but doing so while the installation is
    # potentially being done through Teleport would result in everything getting
    # summarily killed, so the best we can do is let the unhealthy schedule detector
    # trigger a restart at a later, potentially inopportune time.
    systemctl enable teleport-upgrade.timer
    systemctl start teleport-upgrade.timer
fi
