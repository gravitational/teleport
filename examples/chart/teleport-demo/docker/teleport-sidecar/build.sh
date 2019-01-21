#!/bin/bash
docker build -t teleport-sidecar:3.1.4 --build-arg TELEPORT_VERSION=3.1.4 .