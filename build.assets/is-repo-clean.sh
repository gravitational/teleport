#!/bin/bash
#
# Detects whether the repository is clean.
#

GIT="${GIT:-git}"

main() {
  if ! command -v "$GIT" >/dev/null; then
    echo "Unable to find git command."
    exit 1
  fi
  
  if ! "$GIT" diff --quiet; then
    echo "The repository is dirty."
    exit 1
  fi
}

main "$@"
