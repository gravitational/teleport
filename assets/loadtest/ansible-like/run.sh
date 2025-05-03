#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

mkdir -p /run/user/1000/ssh-control

exec dumb-init xargs -P 0 -I % ./run-node.sh % < state/inventory
