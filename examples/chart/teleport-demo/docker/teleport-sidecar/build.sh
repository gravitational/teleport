#!/bin/bash
set -e
ONE=$1
TWO=$2
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
GCPROJECT=kubeadm-167321
if [[ "" != "${TWO}" ]]; then
    GCPROJECT=${TWO}
    shift
fi
echo "${VERSION} version"
echo "${GCPROJECT} project"

docker build --pull \
    -t gcr.io/${GCPROJECT}/teleport-sidecar:${VERSION} \
    -t gcr.io/${GCPROJECT}/teleport-sidecar:latest \
    --cache-from gcr.io/${GCPROJECT}/teleport-sidecar:latest \
    --build-arg TELEPORT_VERSION=${VERSION} \
    . 
docker push gcr.io/${GCPROJECT}/teleport-sidecar:${VERSION}
docker push gcr.io/${GCPROJECT}/teleport-sidecar:latest
