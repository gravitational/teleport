#!/usr/bin/env bash
# tsh.sh — run a tsh command as a specific actor against the cluster.
# Each actor gets a persistent named volume holding their tsh profile state,
# so subsequent calls preserve login state across invocations.
#
# Usage: tsh.sh <project> <actor> <tsh-version> <tsh-args...>

set -euo pipefail

PROJECT="${1:?usage: tsh.sh <project> <actor> <version> <tsh-args...>}"; shift
ACTOR="${1:?usage: tsh.sh <project> <actor> <version> <tsh-args...>}"; shift
VERSION="${1:?usage: tsh.sh <project> <actor> <version> <tsh-args...>}"; shift

VOLUME="${PROJECT}-${ACTOR}-home"
docker volume create "$VOLUME" >/dev/null

exec docker run --rm \
  --network "${PROJECT}-net" \
  -v "${VOLUME}:/home/teleport" \
  -e HOME=/home/teleport \
  --user 0:0 \
  --entrypoint=/usr/local/bin/tsh \
  "public.ecr.aws/gravitational/teleport-ent-distroless:${VERSION}" \
  "$@"
