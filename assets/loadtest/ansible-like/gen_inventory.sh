#!/bin/sh
cd "$( dirname -- "${0}" )" || exit 1

tsh -i tbot_destdir_id/identity --proxy PROXYHOST:443 ls --format=json > inventory.json
# jq -r '.[] | select(.metadata.expires > (now | strftime("%Y-%m-%dT%H:%M:%SZ"))) | .metadata.name + ".CLUSTERNAME"' < inventory.json | sort -R > inventory
jq -r '.[] | select(.metadata.expires > (now | strftime("%Y-%m-%dT%H:%M:%SZ"))) | .spec.hostname + ".CLUSTERNAME"' < inventory.json | sort -R > inventory
wc -l inventory
