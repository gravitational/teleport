#!/bin/sh
cd "$( dirname -- "${0}" )" || exit 1

mkdir -p /run/user/1000/ssh-control

exec dumb-init xargs -P 0 -I % ./run_node.sh % < inventory
