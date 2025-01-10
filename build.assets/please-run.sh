#!/bin/sh

# This script is a helper that tells developers what generated content is out of date
# and which command to run.
# When running on GitHub actions, the script will also create an error in the PR and
# collapse the diff to improve readability.

set -eu

# only echoes the string if we are in GitHub Actions
echo_gha() {
  [ -n "${GITHUB_ACTIONS+x}" ] && echo "$@"
}

main() {
  if [ $# -ne 2 ]; then
    echo "Usage: $0 <kind> <generate command>" >&2
    exit 1
  fi

  KIND="$1"
  GENERATE_COMMAND="$2"

  TITLE="$KIND are out-of-date"
  MESSAGE="Please run the command \`$GENERATE_COMMAND\`"

  # Create a GitHub error
  echo_gha "::error file=Makefile,title=$TITLE::$MESSAGE"

  echo "============="
  echo "$TITLE"
  echo "$MESSAGE"
  echo "============="

  echo_gha "::group::Diff output"
  git diff || true
  echo_gha "::endgroup::"
}

main "$@"