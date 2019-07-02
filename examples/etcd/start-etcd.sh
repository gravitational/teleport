#!/bin/bash
#
# Example of how etcd must be started in the full TLS mode, i.e.
#   - server cert is checked by clients
#   - client cert is checked by the server
#
# NOTE: this file is also used to run etcd tests.
#

EXAMPLES_DIR=$GOPATH/src/github.com/gravitational/teleport/examples/etcd

HERE=$(readlink -f $0)
cd $(dirname $HERE)

mkdir -p data
etcd --name teleportstorage \
     --data-dir data/etcd \
     --initial-cluster-state new \
     --cert-file=$EXAMPLES_DIR/certs/server-cert.pem \
     --key-file=$EXAMPLES_DIR/certs/server-key.pem \
     --trusted-ca-file=$EXAMPLES_DIR/certs/ca-cert.pem \
     --advertise-client-urls=https://127.0.0.1:2379 \
     --listen-client-urls=https://127.0.0.1:2379 \
     --client-cert-auth
