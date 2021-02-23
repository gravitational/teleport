#!/bin/bash
export TELEPORT_CONFIG_PATH=$(mktemp -t teleport-generate-configXXXXXXXX)
export TELEPORT_CONFD_DIR=$(mktemp -d -t teleport.conf.dXXXXXXXX)
