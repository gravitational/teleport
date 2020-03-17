#!/bin/bash
set -e
TWO=$2
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
GCPROJECT=kubeadm-167321
if [[ "${TWO}" != "" ]]; then
    GCPROJECT=${TWO}
    shift
fi


docker pull quay.io/gravitational/debian-grande:buster
docker build --pull \
    -t gcr.io/${GCPROJECT}/namespace-cleaner:${VERSION} \
    -t gcr.io/${GCPROJECT}/namespace-cleaner:latest \
    --cache-from quay.io/gravitational/debian-grande:buster,gcr.io/${GCPROJECT}kubeadm-167321/namespace-cleaner:latest \
    . 
docker push gcr.io/${GCPROJECT}/namespace-cleaner:${VERSION}
docker push gcr.io/${GCPROJECT}/namespace-cleaner:latest
