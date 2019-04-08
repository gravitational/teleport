#!/usr/bin/env bash
set -e
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
docker pull gcr.io/kubeadm-167321/teleport-ent:latest || true # pull down latest built version to use as build cache, don't error if not present
docker build \
    -t gcr.io/kubeadm-167321/teleport-ent:${VERSION} \
    -t gcr.io/kubeadm-167321/teleport-ent:latest \
    --cache-from gcr.io/kubeadm-167321/teleport-ent:latest \
    --build-arg TELEPORT_VERSION=${VERSION} . $*
docker push gcr.io/kubeadm-167321/teleport-ent:${VERSION}
docker push gcr.io/kubeadm-167321/teleport-ent:latest
