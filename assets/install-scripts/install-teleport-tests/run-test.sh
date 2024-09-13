#!/bin/bash
# $1: Teleport version (e.g., 10.11.12)
# $2: edition: cloud, team, oss, or enterprise
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

function test_teleport_version() {
    VER=$(teleport version);
    if [ "$VER" -ne $1 ]; then
      echo "INSTALL_SCRIPT_TEST_FAILURE: expected teleport to have version $VER but got $1"
    fi
}

echo "RUNNING TEST"

if [ "$#" -lt 2 ]; then 
  echo "ERROR: There must be two parameters in run-test.sh: <VERSION> <EDITION>";
  exit 1
fi

if ! echo "$1" |  grep -qE "[0-9]+\.[0-9]+\.[0-9]+"; then
  echo "ERROR: The first parameter must be a version number, e.g., 10.1.9.";
  exit 1
fi

case $2 in
    cloud | team)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_upgrader
    	test_teleport_version
    ;;
    oss)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_teleport_version
    ;;
    enterprise)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_teleport_version
    ;;
    *)
    	echo "INSTALL_SCRIPT_TEST_FAILURE: unsupported edition $2"
    ;;
esac


