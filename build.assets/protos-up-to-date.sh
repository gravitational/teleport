#!/bin/bash
#
# Detects whether protobufs are up to date.
#

CUR_DIR="$(dirname "$0")"
GENPROTO="$CUR_DIR/genproto.sh"

GIT="${GIT:-git}"

if ! command -v "$GIT" >/dev/null; then
  echo "Unable to find git command."
  exit 1
fi

if ! "$GIT" diff --quiet; then
  echo "git state is dirty. Unable to check if the protos are up to date."
  exit 1
fi

# Make sure we reset the working directory.
trap '"$GIT" reset --hard >/dev/null' EXIT

"$GENPROTO"

if ! "$GIT" diff --quiet; then
  echo "Protos are not up to date. Please run make grpc."
  exit 1
fi

echo "Protos are up to date!"
