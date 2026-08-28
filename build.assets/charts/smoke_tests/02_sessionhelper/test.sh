#!/bin/bash

# we only make OCI images for linux, and on all linux platforms the embedded
# session helper should be available and working
docker run --rm --entrypoint /usr/local/bin/teleport --platform $1 $2 -- debug require-session-helper
