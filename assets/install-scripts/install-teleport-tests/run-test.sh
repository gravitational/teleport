#!/bin/bash
# $1: edition: cloud, oss, or enterprise
function test_teleport() {
    if ! type teleport; then
      echo "INSTALL_SCRIPT_TEST_FAILURE: teleport not found"
    fi
}

function test_upgrader() {
    if ! type teleport-upgrade; then
    	echo "INSTALL_SCRIPT_TEST_FAILURE: teleport-upgrade not found"
    fi
}

echo "RUNNING TEST"

case $1 in
    cloud)
      test_upgrader
      ;;&
    oss | enterprise | cloud)
      test_teleport
      ;;
    *)
    	echo "INSTALL_SCRIPT_TEST_FAILURE: unsupported edition $1"
    ;;
esac


