#!/bin/bash
VERSION=3.1.4
if [[ "$1" != "" ]]; then
    VERSION=$1
fi
for f in *; do
    if [[ -d $f ]]; then
        pushd $f
        ./build.sh ${VERSION}
        popd
    fi
done