#!/usr/bin/env bash
set -e
VERSION=3.2.0
if [[ "$1" != "" ]]; then
    VERSION=$1
    shift
fi
docker pull quay.io/gravitational/debian-grande:buster
docker build --pull \
    -t gcr.io/kubeadm-167321/cloudflare-agent:${VERSION} \
    -t gcr.io/kubeadm-167321/cloudflare-agent:latest \
    --cache-from quay.io/gravitational/debian-grande:buster,gcr.io/kubeadm-167321/cloudflare-agent:latest \
    . $*
docker push gcr.io/kubeadm-167321/cloudflare-agent:${VERSION}
docker push gcr.io/kubeadm-167321/cloudflare-agent:latest
