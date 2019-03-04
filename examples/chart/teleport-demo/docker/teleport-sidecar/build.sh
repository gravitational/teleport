#!/usr/bin/env bash
set -e
VERSION=3.1.7
if [[ "$1" != "" ]]; then
    VERSION=$1
fi
docker pull gcr.io/kubeadm-167321/teleport-sidecar:${VERSION} || true # in case image hasn't already been built
docker build -t gcr.io/kubeadm-167321/teleport-sidecar:${VERSION} --build-arg TELEPORT_VERSION=${VERSION} .
docker push gcr.io/kubeadm-167321/teleport-sidecar:${VERSION}