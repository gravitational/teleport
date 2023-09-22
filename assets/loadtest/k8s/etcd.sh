#!/bin/sh
# This script runs etcd.
set -e
set -x

PEERS="etcd-0=https://etcd-0.etcd:2380,etcd-1=https://etcd-1.etcd:2380,etcd-2=https://etcd-2.etcd:2380"
exec etcd \
  --name ${POD_NAME} \
  --advertise-client-urls https://${POD_NAME}.etcd:2379 \
  --listen-client-urls https://0.0.0.0:2379 \
  --initial-advertise-peer-urls https://${POD_NAME}.etcd:2380 \
  --listen-peer-urls https://0.0.0.0:2380 \
  --initial-cluster ${PEERS} \
  --trusted-ca-file=/etc/etcd/certs/ca-cert.pem \
  --cert-file=/etc/etcd/certs/server-cert.pem \
  --key-file=/etc/etcd/certs/server-key.pem \
  --peer-cert-file=/etc/etcd/certs/server-cert.pem \
  --peer-key-file=/etc/etcd/certs/server-key.pem \
  --peer-trusted-ca-file=/etc/etcd/certs/ca-cert.pem \
  --client-cert-auth \
  --peer-client-cert-auth \
  --auto-compaction-retention=1