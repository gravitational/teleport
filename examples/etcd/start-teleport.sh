#!/bin/bash
#
# Example of how Teleport must be started to connect to etcd
HERE=$(readlink -f "$0")
# shellcheck disable=SC2046
cd $(dirname "$HERE") || exit

teleport start -c teleport.yaml -d
