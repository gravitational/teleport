#!/bin/bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <version> (e.g., 17.5.1)"
  exit 1
fi

VERSION="$1"
if [[ "${VERSION}" != v* ]]; then
  VERSION="v${VERSION}"
fi

BRANCH="release/$VERSION"
PATCH_FILE="loadsmart.patch"
FILES_TO_PATCH=".circleci/config.yml Makefile build.assets/Makefile"
UPSTREAM_URL="https://github.com/gravitational/teleport.git"

cd "$(git rev-parse --show-toplevel)"

echo "> Adding upstream remote..."
if ! git remote get-url upstream &>/dev/null; then
  git remote add upstream ${UPSTREAM_URL}
fi

echo "> Fetching upstream..."
git fetch upstream --no-tags
git fetch upstream +refs/tags/"${VERSION}":refs/tags/"${VERSION}" --no-tags

echo "Creating new branch from upstream tag: ${VERSION}..."
git checkout tags/"${VERSION}" -B "${BRANCH}"

echo "> Generating patch from loadsmart..."
git format-patch "$(git merge-base upstream/master master)"..master --stdout -- ${FILES_TO_PATCH} >"${PATCH_FILE}"

echo "> Applying patch..."
git am $PATCH_FILE

echo "> Tagging patched release as $VERSION..."
git tag "${VERSION}" --force
git push origin "${BRANCH}"
git push origin refs/tags/"${VERSION}" --force

echo "> Push complete. A CircleCI pipeline should now be triggered at: https://app.circleci.com/pipelines/github/loadsmart/teleport?branch=${BRANCH}"
