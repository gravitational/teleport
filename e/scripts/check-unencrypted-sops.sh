#!/usr/bin/env bash

# This is a pre-commit hook that will attempt to verify that all SOPS files
# have been encrypted.

fail=false
# For each SOPS file in the commit
while read -r file_name; do
    # Check for the sops YAML key
    grep -q 'sops:' "${file_name}" && continue

    echo "SOPS file '${file_name}' is not encrypted!!" >&2
    fail=true
done < <(git diff --cached --name-only --diff-filter=d -- '*?.sops.yaml')

[[ "${fail}" == 'false' ]] || exit 1
