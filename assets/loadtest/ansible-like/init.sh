#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

source vars.env

if systemctl is-active -q tbot.service; then
    echo "stopping extant tbot.service..." >&2
    sudo systemctl stop tbot.service
fi

sudo mkdir -p /etc/tbot

sudo mkdir -p /var/lib/teleport/bot

sudo chown -R "${BOT_USER:?}:${BOT_USER:?}" /var/lib/teleport/bot

sudo mkdir -p /opt/machine-id

sudo chown -R "${BOT_USER:?}:${BOT_USER:?}" /opt/machine-id


echo "generating tbot config..." >&2

sudo tee /etc/tbot.yaml > /dev/null <<EOF
version: v2
proxy_server: ${PROXY_HOST:?}:${PROXY_PORT:?}
diag_addr: "0.0.0.0:3000"
onboarding:
  join_method: token
  token: ${BOT_TOKEN:?}
outputs:
  - type: identity
    destination:
      type: directory
      path: /opt/machine-id
storage:
  type: directory
  path: /var/lib/teleport/bot
services:
  - type: ssh-multiplexer
    destination:
      type: directory
      path: /opt/machine-id
    enable_resumption: true
    proxy_command:
      - fdpass-teleport
    proxy_templates_path: /etc/tbot/proxy-templates.yaml
EOF


echo "generating proxy templates..." >&2

sudo tee /etc/tbot/proxy-templates.yaml > /dev/null <<EOF
proxy_templates:
  - template: "^(.*).${PROXY_HOST:?}:[0-9]+$" # <nodename>.<clustername>:<port>
    query: 'contains(split(labels.NODENAME, ","), "\$1")'
EOF


echo "installing tbot systemd unit..." >&2

sudo tbot install systemd --write --force --config /etc/tbot.yaml --user "${BOT_USER:?}" --group "${BOT_USER:?}"


echo "starting tbot.service..." >&2

sudo systemctl daemon-reload

sudo systemctl start tbot.service
