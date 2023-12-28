#!/bin/bash

set -e

mkdir -p toolchains
DOCKER_BUILDKIT=0 BUILDKIT_PROGRESS=plain docker build -t teleport-builder-base -f Dockerfile-ct-ng .

docker volume create toolchain

docker run --rm -v toolchain:/toolchain busybox \
  /bin/sh -c 'touch /toolchain/.initialized && chown -R 1000:1000 /toolchain'

docker volume create 3rdparty

docker run --rm -v 3rdparty:/3rdparty busybox \
  /bin/sh -c 'chown -R 1000:1000 /3rdparty'

docker run -v toolchain:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd amd64 && ct-ng build"
docker run -v toolchain:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd i686 && ct-ng build"
docker run -v toolchain:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd arm64 && ct-ng build"
docker run -v toolchain:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd arm && ct-ng build"
