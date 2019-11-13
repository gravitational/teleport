#!/usr/bin/env bash
set -e
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
docker pull quay.io/gravitational/debian-grande:buster
docker build --pull \
    -t gcr.io/kubeadm-167321/namespace-cleaner:${VERSION} \
    -t gcr.io/kubeadm-167321/namespace-cleaner:latest \
    --cache-from quay.io/gravitational/debian-grande:buster,gcr.io/kubeadm-167321/namespace-cleaner:latest \
    . $*
docker push gcr.io/kubeadm-167321/namespace-cleaner:${VERSION}
docker push gcr.io/kubeadm-167321/namespace-cleaner:latest
