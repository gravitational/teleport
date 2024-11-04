#!/usr/bin/env bash

set -e

CURRENT_DIR="$(dirname "$0")"
"${CURRENT_DIR}/check-unencrypted-sops.sh"
