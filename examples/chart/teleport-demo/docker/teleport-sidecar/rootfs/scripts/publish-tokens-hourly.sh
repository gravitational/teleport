#!/usr/bin/env bash
set -e
# run at startup, then every hour after that
while true; do
    date
    /usr/bin/teleport-publish-tokens
    sleep 3600
done