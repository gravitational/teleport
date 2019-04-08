#!/usr/bin/env bash
set -e
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
docker build --pull \
    -t gcr.io/kubeadm-167321/teleport-sidecar:${VERSION} \
    -t gcr.io/kubeadm-167321/teleport-sidecar:latest \
    --cache-from gcr.io/kubeadm-167321/teleport-sidecar:latest \
    --build-arg TELEPORT_VERSION=${VERSION} \
    . $*
docker push gcr.io/kubeadm-167321/teleport-sidecar:${VERSION}
docker push gcr.io/kubeadm-167321/teleport-sidecar:latest