#!/bin/bash
VERSION=3.1.4
if [[ "$1" != "" ]]; then
    VERSION=$1
fi
docker build -t gcr.io/kubeadm-167321/teleport-sidecar:${VERSION} --build-arg TELEPORT_VERSION=${VERSION} .
docker push gcr.io/kubeadm-167321/teleport-sidecar:${VERSION}