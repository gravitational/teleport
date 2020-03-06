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

docker pull quay.io/gravitational/debian-grande:buster
echo "docker build --pull \
    -t gcr.io/${GCPROJECT}/cloudflare-agent:${VERSION} \
    -t gcr.io/${GCPROJECT}/cloudflare-agent:latest \
    --cache-from quay.io/gravitational/debian-grande:buster,gcr.io/${GCPROJECT}/cloudflare-agent:latest \
    .  "
docker build --pull \
    -t gcr.io/${GCPROJECT}/cloudflare-agent:${VERSION} \
    -t gcr.io/${GCPROJECT}/cloudflare-agent:latest \
    --cache-from quay.io/gravitational/debian-grande:buster,gcr.io/${GCPROJECT}/cloudflare-agent:latest \
    .   
docker push gcr.io/${GCPROJECT}/cloudflare-agent:${VERSION}
docker push gcr.io/${GCPROJECT}/cloudflare-agent:latest

