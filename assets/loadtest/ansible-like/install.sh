#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

source vars.env

mkdir -p state

cd state

echo "installing teleport..." >&2

wget -q "https://${TELEPORT_CDN:?}/${TELEPORT_ARTIFACT:?}"

tar -xf "${TELEPORT_ARTIFACT:?}"

rm "${TELEPORT_ARTIFACT:?}"

sudo ./teleport/install

echo "installing fdpass-teleport..." >&2

sudo cp ./teleport/fdpass-teleport "$(dirname "$(which teleport)")"

rm -rf ./teleport


echo "installing dumb-init..." >&2

sudo wget -q -O /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_x86_64

sudo chmod +x /usr/local/bin/dumb-init
