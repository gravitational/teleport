#!/bin/bash
docker run --rm --entrypoint /usr/bin/teleport --platform $1 $2 -- version