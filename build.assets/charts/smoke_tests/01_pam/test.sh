#!/bin/bash

# Given an ssh node configuration with use_pam_auth enabled, Teleport
# will test if PAM is available. If PAM is not installed correctly, 
# Teleport will exit immediately with a nonzero status. 
#
# If teleport is still up when the timeout expires, then we're 
# probably OK
#
# A timeout of 20s is set to account for potentially slow startup times.
# QEMU eumlation may slow startup time for some platforms and during that
# time the process won't shutdown gracefully resulting in a nonzero code.
timeout --preserve-status 20s docker run --platform $1 --rm --entrypoint /usr/local/bin/teleport -v "$(pwd):/etc/teleport" $2 start -c /etc/teleport/config.yaml
