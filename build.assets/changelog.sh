#!/bin/bash
#
# This script generates a PR diff between the provided base tag and the tip of
# the specified base branch.
set -eu

COMMIT=$(git rev-list -n 1 v$BASE_TAG)

DATE=$(git show -s --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd $COMMIT)

gh pr list \
    --search "base:$BASE_BRANCH merged:>$DATE -label:no-changelog" \
    --limit 200 \
    --json number,body,url \
    --jq 'map(.entry = (.body | capture("changelog:\\s*(?<changelog>.*[^\\r\\n])"; "i") .changelog)) | .[] | "* \(.entry) [#\(.number)](\(.url))"'
