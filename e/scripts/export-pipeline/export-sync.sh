#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: export-sync.sh --branch <branch> [options]

Sync an already-approved export branch to the OSS mirror and advance checkpoint.

Options:
  --branch <branch>        Source branch name (for example: master)
  --remote <remote>        Git remote holding source/export refs (default: origin)
  --oss-remote <remote>    Git remote for the OSS mirror (default: teleport-oss)
  -h, --help               Show this help text

Notes:
  - Assumes export/<branch> already contains the approved sanitized history.
  - Advances export-checkpoint/<branch> to the current source branch tip only after OSS push succeeds.
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

find_approved_source_commit() {
  local export_ref=$1
  local source_commit

  source_commit=$(git log \
    --topo-order \
    --max-count=1 \
    --grep='^Export-Source-Commit: ' \
    --format='%(trailers:key=Export-Source-Commit,valueonly)' \
    "$export_ref")

  [[ -n "$source_commit" ]] || die "approved export branch contains no source provenance trailer"
  [[ "$source_commit" != *$'\n'* ]] || die "approved export commit contains multiple source provenance trailers"
  if [[ ! "$source_commit" =~ ^[0-9a-f]{40}$ && ! "$source_commit" =~ ^[0-9a-f]{64}$ ]]; then
    die "approved export commit has an invalid source provenance value: $source_commit"
  fi

  printf '%s\n' "$source_commit"
}

BRANCH=
REMOTE=origin
OSS_REMOTE=teleport-oss

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)
      BRANCH=${2:-}
      shift 2
      ;;
    --remote)
      REMOTE=${2:-}
      shift 2
      ;;
    --oss-remote)
      OSS_REMOTE=${2:-}
      shift 2
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

require_cmd git

if [[ -n $(git status --porcelain) ]]; then
  die "worktree is dirty; rerun in a clean clone"
fi

SRC_REF="$BRANCH"
EXP_REF="export/$BRANCH"
CHECKPOINT_TAG="export-checkpoint/$BRANCH"

git fetch "$REMOTE"
git fetch "$REMOTE" "+refs/tags/$CHECKPOINT_TAG:refs/tags/$CHECKPOINT_TAG"

# Verify the required refs exist
git show-ref --verify --quiet "refs/remotes/$REMOTE/$SRC_REF" || die "missing remote source branch: $REMOTE/$SRC_REF"
git show-ref --verify --quiet "refs/remotes/$REMOTE/$EXP_REF" || die "missing remote export branch: $REMOTE/$EXP_REF"
git show-ref --verify --quiet "refs/tags/$CHECKPOINT_TAG" || die "missing checkpoint tag: $CHECKPOINT_TAG"

APPROVED_SOURCE_COMMIT=$(find_approved_source_commit "$REMOTE/$EXP_REF")
CURRENT_CHECKPOINT_COMMIT=$(git rev-parse "$CHECKPOINT_TAG^{commit}")
git cat-file -e "$APPROVED_SOURCE_COMMIT^{commit}" 2>/dev/null || die "source provenance commit does not exist locally: $APPROVED_SOURCE_COMMIT"
git merge-base --is-ancestor "$CURRENT_CHECKPOINT_COMMIT" "$APPROVED_SOURCE_COMMIT" || die "source provenance commit is behind or unrelated to $CHECKPOINT_TAG"
git merge-base --is-ancestor "$APPROVED_SOURCE_COMMIT" "$REMOTE/$SRC_REF" || die "source provenance commit is not an ancestor of $REMOTE/$SRC_REF"

echo "Approved $REMOTE/$EXP_REF maps to source commit $APPROVED_SOURCE_COMMIT"

# Push the exact approved remote-tracking export ref. This avoids depending on
# or mutating a potentially stale local export branch.
git push "$OSS_REMOTE" "refs/remotes/$REMOTE/$EXP_REF:refs/heads/$BRANCH"

# Advance the checkpoint only to the newest source commit represented in the
# approved export history. The checkpoint may safely lag when commits are
# removed entirely by the export filter.
if [[ "$CURRENT_CHECKPOINT_COMMIT" == "$APPROVED_SOURCE_COMMIT" ]]; then
  checkpoint_status="left $CHECKPOINT_TAG unchanged"
else
  git tag -f "$CHECKPOINT_TAG" "$APPROVED_SOURCE_COMMIT"
  git push "$REMOTE" "refs/tags/$CHECKPOINT_TAG" --force
  checkpoint_status="advanced $CHECKPOINT_TAG to $APPROVED_SOURCE_COMMIT"
fi

echo "Synced $EXP_REF to $OSS_REMOTE/$BRANCH and $checkpoint_status"
