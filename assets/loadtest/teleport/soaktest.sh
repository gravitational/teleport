#!/bin/bash
# This script runs the teleport load test soak tests.
set -e
set -x

direct_node=$(tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth -l root ls -f names | grep -v iot)
tunnel_node=$(tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth -l root ls -f names | grep iot)

echo "${direct_node}"
echo "${tunnel_node}"

if [ -z "${direct_node}" ]; then
		echo "no direct dial nodes found to run soak test on.";
		exit 1;
fi

if [ -z "${tunnel_node}" ]; then
		echo "no reverse tunnel nodes found to run soak test on.";
		exit 1;
fi

echo "----Direct Dial Node Test----"
tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" root@"${direct_node}" ls

tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" --interactive root@"${direct_node}" ps aux

echo "----Reverse Tunnel Node Test----"
tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" root@"${tunnel_node}" ls

tsh --insecure --proxy="${PROXY_HOST}":3080 -i /etc/teleport/auth bench --duration="${DURATION}" --interactive root@"${tunnel_node}" ps aux