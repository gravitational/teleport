#!/bin/bash
#
# Example of how etcd must be started in an insecure mode, i.e.
#   - server cert is NOT checked by clients
#   - client cert is NOT checked by the server
#
HERE=$(readlink -f $0)
cd "$(dirname $HERE)" || exit

mkdir -p data
etcd --name teleportstorage \
     --data-dir data/etcd \
     --initial-cluster-state new \
     --advertise-client-urls=http://127.0.0.1:2379 \
     --listen-client-urls=http://127.0.0.1:2379
