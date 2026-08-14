#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: export-tag.sh --branch <branch> --tag <tag> [options]

Map a private lightweight source tag to its sanitized commit and create the
same lightweight tag in the OSS mirror.

Options:
  --branch <branch>        Source and OSS branch containing the tagged commit
  --tag <tag>              Private source tag to export
  --remote <remote>        Git remote holding source/export refs (default: origin)
  --oss-remote <remote>    Git remote for the OSS mirror (default: teleport-oss)
  --check                  Validate readiness without creating the OSS tag
  -h, --help               Show this help text

Notes:
  - The private source tag must be lightweight.
  - The tagged source commit must have an exact Export-Source-Commit match in
    export/<branch> and that sanitized commit must already be on the OSS branch.
  - The private tag is canonical. A mismatched OSS tag is moved to the expected
    sanitized commit using force-with-lease.
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

BRANCH=
TAG=
REMOTE=origin
OSS_REMOTE=teleport-oss
CHECK_ONLY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)
      [[ $# -ge 2 && -n ${2:-} ]] || die "--branch requires a value"
      BRANCH=$2
      shift 2
      ;;
    --tag)
      [[ $# -ge 2 && -n ${2:-} ]] || die "--tag requires a value"
      TAG=$2
      shift 2
      ;;
    --remote)
      [[ $# -ge 2 && -n ${2:-} ]] || die "--remote requires a value"
      REMOTE=$2
      shift 2
      ;;
    --oss-remote)
      [[ $# -ge 2 && -n ${2:-} ]] || die "--oss-remote requires a value"
      OSS_REMOTE=$2
      shift 2
      ;;
    --check)
      CHECK_ONLY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -n "$BRANCH" ]] || {
  usage >&2
  die "--branch is required"
}
[[ -n "$TAG" ]] || {
  usage >&2
  die "--tag is required"
}

SRC_REF="$BRANCH"
EXP_REF="export/$BRANCH"
SOURCE_TAG_REF="refs/tags/$TAG"
OSS_BRANCH_REF="refs/heads/$BRANCH"
OSS_TAG_REF="refs/tags/$TAG"
OSS_TRACKING_REF="refs/remotes/$OSS_REMOTE/$BRANCH"

git fetch "$REMOTE"

# Verify that the source tag exists and points to a commit reachable from the source branch.
SOURCE_TAG_COMMIT=$(git ls-remote --refs "$REMOTE" "$SOURCE_TAG_REF" | awk 'NR == 1 { print $1 }')
[[ -n "$SOURCE_TAG_COMMIT" ]] || die "missing source tag: $REMOTE/$TAG"
git merge-base --is-ancestor "$SOURCE_TAG_COMMIT" "$REMOTE/$SRC_REF" || die "source tag $REMOTE/$TAG does not identify a commit reachable from $REMOTE/$SRC_REF"

# Match a commit in the OSS mirror to the given tag in this repo.
# Exported commits in the OSS mirror are annotated with an Export-Source-Commit 
# trailer that identifies the original source commit.
EXPORTED_COMMIT=$(git log \
  --format='%H%x09%(trailers:key=Export-Source-Commit,valueonly)' \
  "$REMOTE/$EXP_REF" |
  awk -F $'\t' -v source="$SOURCE_TAG_COMMIT" '$2 == source { print $1 }')
[[ -n "$EXPORTED_COMMIT" ]] || die "source tag $TAG has no provenance match in $REMOTE/$EXP_REF; export and merge that source commit before tagging OSS"
git fetch "$OSS_REMOTE" "+$OSS_BRANCH_REF:$OSS_TRACKING_REF"
git merge-base --is-ancestor "$EXPORTED_COMMIT" "$OSS_TRACKING_REF" || die "sanitized commit $EXPORTED_COMMIT is not present on $OSS_REMOTE/$BRANCH; sync the approved export before tagging"
echo "Source tag $REMOTE/$TAG maps $SOURCE_TAG_COMMIT to sanitized commit $EXPORTED_COMMIT"

# Check whether the canonical tag is already present in OSS.
EXISTING_OSS_TAG=$(git ls-remote --refs "$OSS_REMOTE" "$OSS_TAG_REF" | awk 'NR == 1 { print $1 }')
if [[ "$EXISTING_OSS_TAG" == "$EXPORTED_COMMIT" ]]; then
  echo "OSS tag $OSS_REMOTE/$TAG already points to $EXPORTED_COMMIT"
  exit 0
fi

if [[ $CHECK_ONLY -eq 1 ]]; then
  if [[ -n "$EXISTING_OSS_TAG" ]]; then
    echo "Ready to update lightweight OSS tag $OSS_REMOTE/$TAG from $EXISTING_OSS_TAG to $EXPORTED_COMMIT"
  else
    echo "Ready to create lightweight OSS tag $OSS_REMOTE/$TAG at $EXPORTED_COMMIT"
  fi
  exit 0
fi

if [[ -n "$EXISTING_OSS_TAG" ]]; then
  git push --force-with-lease="$OSS_TAG_REF:$EXISTING_OSS_TAG" \
    "$OSS_REMOTE" "$EXPORTED_COMMIT:$OSS_TAG_REF"
  echo "Updated lightweight OSS tag $OSS_REMOTE/$TAG from $EXISTING_OSS_TAG to $EXPORTED_COMMIT"
else
  git push "$OSS_REMOTE" "$EXPORTED_COMMIT:$OSS_TAG_REF"
  echo "Created lightweight OSS tag $OSS_REMOTE/$TAG at $EXPORTED_COMMIT"
fi
