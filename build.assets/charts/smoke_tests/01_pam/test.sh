#!/bin/bash

# Given an ssh node configuration with use_pam_auth enabled, Teleport
# will test if PAM is available. If PAM is not installed correctly, 
# Teleport will exit immediately with a nonzero status. 
#
# If teleport is still up when the timeout expires, then we're 
# probably OK
timeout --preserve-status 10s docker run --platform $1 --rm --entrypoint /usr/local/bin/teleport -v "$(pwd):/etc/teleport" $2 start -c /etc/teleport/config.yaml