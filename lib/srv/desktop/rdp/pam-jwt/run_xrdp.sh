#!/usr/bin/env bash

docker build -t xrdp-teleport .
docker rm -f xrdp-teleport
docker run -d --name xrdp-teleport -p 33890:3389 --device /dev/fuse --cap-add SYS_ADMIN --init xrdp-teleport