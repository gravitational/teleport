#!/bin/bash

# Determine the directory the script was run from.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

echo "Generating bot ID"
BOT_ID=$(tr -dc a-z0-9 </dev/urandom | head -c 6)

echo "Generating bot token"
cd ${SCRIPT_DIR}/../auth
TCTL_OUTPUT=$(tctl -c teleport.yaml bots add static-${BOT_ID} --roles=access --logins=root --format=json)
if [ $? -ne 0 ]; then
  echo "Failed to retrieve a new bot token, is the Auth server running?"
  exit 1
fi

TOKEN=$(echo "${TCTL_OUTPUT}" | jq -r .token_id)
if [ $? -ne 0 ]; then
  echo "Failed to parse tctl output, do you have jq installed?"
  exit 1
fi

# Return to tbot directory
cd -

echo "Generating Config file using \`tbot configure\`"
tbot configure \
  --auth-server localhost:5000 \
  --oneshot \
  --join-method "token" \
  --token ${TOKEN} \
  --data-dir ".data" \
  --insecure \
  --debug \
  -o "${SCRIPT_DIR}/tbot.yaml"

if [ $? -ne 0 ]; then
  echo "Failed to generate a valid tbot configuration file using configure."
  exit 1
fi

tbot start -c tbot.yaml -d
