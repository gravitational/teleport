#!/bin/bash

set -eu

# reload systemd configuration and start/restart units
systemctl daemon-reload
systemctl try-restart teleport.service
systemctl start teleport-upgrade.timer
