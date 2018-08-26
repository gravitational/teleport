#!/bin/bash

etcdctl --ca-file=./certs/ca-cert.pem --cert-file=./certs/client-cert.pem  --key-file=./certs/client-key.pem  --endpoints=https://172.11.1.1:2379 $@

