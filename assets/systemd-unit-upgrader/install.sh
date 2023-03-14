#!/bin/bash

set -euo pipefail

cp teleport-upgrade /usr/local/bin/

cp teleport-upgrade.service /etc/systemd/system/

cp teleport-upgrade.timer /etc/systemd/system/

mkdir -p /etc/systemd/system/teleport.service.d/

cp teleport-upgrader-env.conf /etc/systemd/system/teleport.service.d/

systemctl daemon-reload

systemctl try-restart teleport.service

systemctl start teleport-upgrade.timer
