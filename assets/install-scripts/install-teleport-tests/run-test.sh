#!/bin/bash
# Script parameters:
# $1: Teleport version (e.g., 10.11.12)
# $2: edition: cloud, oss, or enterprise

function test_teleport() {
    if ! type teleport; then
      echo "INSTALL_SCRIPT_TEST_FAILURE: teleport not found"
    fi
}

function test_tctl() {
    if ! type tctl; then
    	echo "INSTALL_SCRIPT_TEST_FAILURE: tctl not found"
    fi
}

function test_tsh() {
    if ! type tsh; then
    	echo "INSTALL_SCRIPT_TEST_FAILURE: tsh not found"
    fi
}

function test_tbot() {
    if ! type tctl; then
    	echo "INSTALL_SCRIPT_TEST_FAILURE: tbot not found"
    fi
}

function test_upgrader() {
    if ! type teleport-upgrade; then
    	echo "INSTALL_SCRIPT_TEST_FAILURE: upgrader not found"
    fi
}

# $1: Expected version
function test_teleport_version() {
    VER=$(teleport version | sed -E 's|.*Teleport (Enterprise )?v([0-9]+\.[0-9]+\.[0-9]+).*|\2|');
    if [ "$VER" != "$1" ]; then
      echo "INSTALL_SCRIPT_TEST_FAILURE: expected teleport to have version $1 but got $VER"
    fi
}

echo "RUNNING TEST"

if [ "$#" -lt 2 ]; then 
  echo "ERROR: There must be two parameters in run-test.sh: <VERSION> <EDITION>";
  exit 1
fi

if ! echo "$1" | grep -qE "[0-9]+\.[0-9]+\.[0-9]+"; then
  echo "ERROR: The first parameter must be a version number, e.g., 10.1.9.";
  exit 1
fi

case $2 in
    cloud)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_upgrader
	# Don't test the expected version for Teleport Enterprise (Cloud), since
	# the test suite does not manage the version of the stable/cloud
	# channel.
    ;;
    oss)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_teleport_version $1
    ;;
    enterprise)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_teleport_version $1
    ;;
    *)
    	echo "INSTALL_SCRIPT_TEST_FAILURE: unsupported edition $2"
    ;;
esac


