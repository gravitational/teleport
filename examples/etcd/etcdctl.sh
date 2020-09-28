#!/bin/bash

HERE=$(readlink -f $0)
cd "$(dirname $HERE)" || exit

ETCDCTL_API=3 etcdctl --cacert=./certs/ca-cert.pem --cert=./certs/client-cert.pem  --key=./certs/client-key.pem  --endpoints=https://127.0.0.1:2379 "$@"

