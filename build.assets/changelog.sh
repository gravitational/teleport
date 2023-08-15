#!/bin/bash
#
# This script generates a PR diff between the provided base tag and the tip of
# the specified base branch.
set -eu

COMMIT=$(git rev-list -n 1 v$BASE_TAG)

DATE=$(git show -s --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd $COMMIT)

gh pr list \
    --search "base:$BASE_BRANCH merged:>$DATE" \
    --limit 200 \
    --json number,title \
    --template "{{range .}}{{printf \"* %v [#%v](https://github.com/gravitational/teleport/pull/%v)\n\" .title .number .number}}{{end}}"
