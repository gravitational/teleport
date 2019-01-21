#!/bin/bash
set -e
set -x
/usr/bin/teleport-replace-node-join-token
echo "ARGS: $@"
#exec teleport start -c /etc/teleport/teleport.yaml "$@"
exec teleport start -c /tmp/teleport.yaml "$@"