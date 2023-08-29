#!/bin/bash

# Determine the directory the script was run from.
SCRIPT_DIR=$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd  )

echo "Generating bot ID"
BOT_ID=$(tr -dc a-z0-9 </dev/urandom | head -c 6)

echo "Generating bot token"
# shellcheck disable=SC2164
cd "${SCRIPT_DIR}/../auth"
TCTL_OUTPUT=$(tctl -c teleport.yaml bots add "static-${BOT_ID}" --roles=access --logins=root --format=json)
# shellcheck disable=SC2181
if [ $? -ne 0 ]; then
  echo "Failed to retrieve a new bot token, is the Auth server running?"
  exit 1
fi

# Return to bot directory
# shellcheck disable=SC2164
# shellcheck disable=SC2103
cd -

TOKEN=$(echo "${TCTL_OUTPUT}" | jq -r .token_id)
# shellcheck disable=SC2181
if [ $? -ne 0 ]; then
  echo "Failed to parse tctl output, do you have jq installed?"
  exit 1
fi

echo "Generating Config file using \`tbot configure\`"
# Disable shellcheck here, because the configure command cannot handle quoted tokens.
# shellcheck disable=SC2086
tbot configure \
  --auth-server localhost:5000 \
  --oneshot \
  --join-method "token" \
  --token ${TOKEN} \
  --data-dir ".data" \
  --insecure \
  --debug \
  -o "${SCRIPT_DIR}/tbot.yaml"

# shellcheck disable=SC2181
if [ $? -ne 0 ]; then
  echo "Failed to generate a valid tbot configuration file using configure."
  exit 1
fi

tbot start -c tbot.yaml -d
