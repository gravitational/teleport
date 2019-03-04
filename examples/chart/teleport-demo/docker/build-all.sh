#!/usr/bin/env bash
set -e
VERSION=3.1.7
if [[ "$1" != "" ]]; then
    VERSION=$1
fi
set -e
for f in *; do
    if [[ -d $f ]]; then
        pushd $f
        ./build.sh ${VERSION}
        popd
    fi
done