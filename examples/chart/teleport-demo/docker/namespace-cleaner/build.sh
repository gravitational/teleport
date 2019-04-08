#!/usr/bin/env bash
set -e
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
docker pull quay.io/gravitational/debian-grande:latest
docker pull gcr.io/kubeadm-167321/namespace-cleaner:latest || true # pull down latest built version to use as build cache, don't error if not present
docker build \
    -t gcr.io/kubeadm-167321/namespace-cleaner:${VERSION} \
    -t gcr.io/kubeadm-167321/namespace-cleaner:latest \
    --cache-from quay.io/gravitational/debian-grande:latest,gcr.io/kubeadm-167321/namespace-cleaner:latest \
    --build-arg TELEPORT_VERSION=${VERSION} . $*
docker push gcr.io/kubeadm-167321/namespace-cleaner:${VERSION}
docker push gcr.io/kubeadm-167321/namespace-cleaner:latest
