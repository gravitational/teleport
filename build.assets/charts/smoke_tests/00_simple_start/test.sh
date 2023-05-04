#!/bin/bash
docker run --rm --entrypoint /usr/local/bin/teleport --platform $1 $2 -- version