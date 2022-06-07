#!/bin/bash
# This script runs the teleport load test soak tests.
set -e
set -x

node=$(tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth -l root ls -f names | grep -v iot)
iot_node=$(tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth -l root ls -f names | grep iot)

echo "${node}"
echo "${iot_node}"

if [ -z "${node}" ]; then
		echo "no regular nodes found to run soak test on.";
		exit 1;
fi

if [ -z "${iot_node}" ]; then
		echo "no IoT nodes found to run soak test on.";
		exit 1;
fi

echo "----Non-IoT Node Test----"
tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" root@"${node}" ls

tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" --interactive root@"${node}" ps aux

echo "----IoT Node Test----"
tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" root@"${iot_node}" ls

tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" --interactive root@"${iot_node}" ps aux