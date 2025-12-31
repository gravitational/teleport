#!/bin/bash

# This script is a dummy example that copies and configures the login load-test agent on many VMs.
# This works by using your current tsh profile to SSH on all VMs labeled with "role:loadtest".
# You are responsible for creating the VMs beforehand and enrolling them in your Teleport cluster.
# You must be allowed to ssh on the VMs as root.
#
# This script can be trivially ported to exec into kubernetes pods, however you must make sure
# that the source IP will be set to the pod and not the underlying node, else 2 pods running
# on the same node will get rate-limited.

if [ -z "$TARGET_CLUSTER" ]; then
  echo "You must set the TARGET_CLUSTER env var"
  exit 1
fi

set -euo pipefail

SYSTEMD_SERVICE="[Unit]
      Description=Login load-test service.

      [Service]
      Type=simple
      ExecStart=/usr/local/bin/loadtest-login --proxy-addr=$TARGET_CLUSTER loadtest-login /root/device.json
      Restart=on-failure
      User=root
"

echo "$SYSTEMD_SERVICE" > loadtest.service

NODES="$(tsh ls -f names role=loadtest)"

# Loop through each node.
for NODE in $NODES; do
	echo "NODE: $NODE"
	tsh ssh "root@$NODE" "apt update && apt install libfido2 ca-certificates -yqq"
	tsh scp ./loadtest-login "root@$NODE:/usr/local/bin/loadtest-login"
	tsh scp ./device.json "root@$NODE:/root/device.json"
	tsh scp ./loadtest.service "root@$NODE:/lib/systemd/system/loadtest.service"
	tsh ssh "root@$NODE" "systemctl daemon-reload"
done;

echo "To start the load-tst, run:"
for NODE in $NODES; do
	echo "tsh ssh root@$NODE systemctl start loadtest"
done;
