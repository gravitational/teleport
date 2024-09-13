#!/bin/bash
# $1: edition: cloud, oss, or enterprise
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

echo "RUNNING TEST"

case $1 in
    cloud)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    	test_upgrader
    ;;
    oss)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    ;;
    enterprise)
    	test_teleport
    	test_tctl
    	test_tsh
    	test_tbot
    ;;
    *)
    	echo "INSTALL_SCRIPT_TEST_FAILURE: unsupported edition $EDITION"
    ;;
esac


