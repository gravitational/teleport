#!/bin/sh

# This script is a helper that tells developers what generated content is out of date
# and which command to run.
# When running on GitHub actions, the script will also create an error in the PR and
# collapse the diff to improve readability.

set -eu

KIND="$1"
GENERATE_COMMAND="$2"

TITLE="$KIND are out-of-date"
MESSAGE="Please run the command \`$GENERATE_COMMAND\`"

if [ -z ${GITHUB_ACTIONS+x} ]; then
    # We are not in GitHub Actions
    echo "============="
    echo "$TITLE"
    echo "$MESSAGE"
    echo "============="

    git diff || true
else
    # We are in GitHub Actions

    # Create a GitHub error
    echo "::error file=Makefile,title=$TITLE::$MESSAGE"

    # Also write to the job logs
    echo "============="
    echo "$TITLE"
    echo "$MESSAGE"
    echo "============="

    echo "::group::Diff output"
    git diff || true
    echo "::endgroup::"
fi