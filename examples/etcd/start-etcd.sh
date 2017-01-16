#!/bin/bash
#
# Example of how etcd must be started in the full TLS mode, i.e.
#   - server cert is checked by clients
#   - client cert is checked by the server
#
mdkir -p data
etcd --name teleportstorage \
     --data-dir data/etcd \
     --initial-cluster-state new \
     --cert-file=certs/server-cert.pem \
     --key-file=certs/server-key.pem \
     --ca-file=certs/ca-cert.pem \
     --advertise-client-urls=https://127.0.0.1:2379 \
     --listen-client-urls=https://127.0.0.1:2379 \
     --client-cert-auth
