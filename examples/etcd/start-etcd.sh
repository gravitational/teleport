#!/bin/bash
#
# Example of how etcd must be started in the full TLS mode, i.e.
#   - server cert is checked by clients
#   - client cert is checked by the server
#
# NOTE: this file is also used to run etcd tests.
#

set -e

# Etcd before v3.5.0 requires ETCD_UNSUPPORTED_ARCH to be set in order to run on arm64.
if [ "$(uname -m)" = "aarch64" ]; then
export ETCD_UNSUPPORTED_ARCH=arm64
fi

HERE=$(readlink -f "$0")
cd "$(dirname "$HERE")" || exit

mkdir -p data
etcd --name teleportstorage \
     --data-dir data/etcd \
     --initial-cluster-state new \
     --cert-file certs/server-cert.pem \
     --key-file certs/server-key.pem \
     --trusted-ca-file certs/ca-cert.pem \
     --advertise-client-urls=https://127.0.0.1:2379 \
     --listen-client-urls=https://127.0.0.1:2379 \
     --client-cert-auth
