#!/bin/bash

set -euo pipefail


dir=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )

role=${1:?'positional argument "role"'}

$dir/ansible/ec2.py --list | jq -r ".${role}[]" | grep -v "^$"
