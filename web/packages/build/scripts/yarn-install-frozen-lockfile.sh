#!/bin/bash

set -ex

# Install JavaScript dependencies and manually check if yarn.lock needs an update.
# Yarn v1 doesn't respect the --frozen-lockfile flag when using workspaces.
# https://github.com/yarnpkg/yarn/issues/4098

message="yarn.lock needs an update. Run yarn install, verify that correct dependencies were \
installed and commit the updated version of yarn.lock. Make sure you have the packages/webapps.e \
submodule initialized and updated first."

cp yarn.lock yarn-before-install.lock
yarn install
git diff --no-index --exit-code yarn-before-install.lock yarn.lock ||
  { echo "$message" ; exit 1; }
