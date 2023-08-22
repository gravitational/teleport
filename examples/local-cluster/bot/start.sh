#!/bin/bash
set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "SCRIPT_DIR: ${SCRIPT_DIR}"

echo "Generating bot ID"
BOT_ID=$(tr -dc a-z0-9 </dev/urandom | head -c 6)

echo "Generating bot token"
cd ${SCRIPT_DIR}/../auth
TOKEN=$(tctl -c teleport.yaml bots add static-${BOT_ID} --roles=access --logins=root --format=json | jq .token_id)
cd -

echo "Resetting config file"
cp tbot.yaml.orig tbot.yaml

echo "Replacing bot token in config"
sed -i "s/__TOKEN__/${TOKEN}/g" tbot.yaml

tbot start -c tbot.yaml -d
