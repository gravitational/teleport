#!/bin/bash
#
# Example of how Teleport must be started to connect to etcd
HERE=$(readlink -f $0)
cd $(dirname $HERE)

teleport start -c teleport.yaml -d
