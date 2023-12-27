#!/bin/bash

set -e

mkdir -p toolchains
DOCKER_BUILDKIT=0 BUILDKIT_PROGRESS=plain docker build -t teleport-builder-base -f Dockerfile-ct-ng .

docker run -v `pwd`/toolchains:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd amd64 && ct-ng build"
docker run -v `pwd`/toolchains:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd i686 && ct-ng build"
docker run -v `pwd`/toolchains:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd arm64 && ct-ng build"
docker run -v `pwd`/toolchains:/home/ctng/x-tools --rm docker.io/library/teleport-builder-base bash -c "cd arm && ct-ng build"
