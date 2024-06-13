#!/bin/sh
cd $( dirname -- ${0} )

tsh -i tbot_destdir_id/identity --proxy xltenant.teleport.sh:443 ls --format=json > inventory.json
# jq -r '.[] | select(.metadata.expires > (now | strftime("%Y-%m-%dT%H:%M:%SZ"))) | .metadata.name + ".xltenant.teleport.sh"' < inventory.json | sort -R > inventory
jq -r '.[] | select(.metadata.expires > (now | strftime("%Y-%m-%dT%H:%M:%SZ"))) | .spec.hostname + ".xltenant.teleport.sh"' < inventory.json | sort -R > inventory
wc -l inventory
