#!/bin/bash
set -euo pipefail

usage() { echo "Usage: $(basename $0) [-w <webapps branch to clone>] [-t <teleport branch to commit to>]" 1>&2; exit 1; }
while getopts ":w:t:" o; do
    case "${o}" in
        w)
            w=${OPTARG}
            ;;
        t)
            t=${OPTARG}
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

if [ -z "${w}" ] || [ -z "${t}" ]; then
    usage
fi

WEBAPPS_BRANCH=${w}
TELEPORT_BRANCH=${t}

# check if gh is installed
if ! type gh >/dev/null 2>&1; then
    echo "The 'gh' utility must be installed to run this script."
    echo "You can download it from https://github.com/cli/cli/releases/latest"
    exit 1
fi

# check if make is installed
if ! type make >/dev/null 2>&1; then
    echo "The 'make' utility must be installed to run this script."
    exit 1
fi

# run 'gh auth status' to check if gh config already exists
if ! gh auth status; then
    # log into github via gh tool
    echo "You're not logged into 'gh' - you will need to complete a Github OAuth flow."
    echo "Choose 'Login with a web browser' if prompted and follow the instructions."
    echo "Choose 'HTTPS' for 'default git protocol' if prompted."
    gh auth login --hostname github.com --web
 fi

# check that the specified remote branches exist
if ! git ls-remote --heads --exit-code git@github.com:gravitational/webapps.git ${WEBAPPS_BRANCH}; then
    echo "Cannot find ${WEBAPPS_BRANCH} in the webapps repo."
    echo "Make sure that the remote branch has been pushed before running this script."
    exit 1
fi
if ! git ls-remote --heads --exit-code git@github.com:gravitational/teleport.git ${TELEPORT_BRANCH}; then
    echo "Cannot find ${TELEPORT_BRANCH} in the teleport repo."
    echo "Make sure that the remote branch has been pushed before running this script."
    exit 1
fi

# keep all the clones in a temp directory
TEMP_DIR="$(mktemp -d)"
pushd $TEMP_DIR

# check that specified branch/commit exists in webapps repo
git clone git@github.com:gravitational/webapps.git webapps
pushd webapps
git fetch --all
# try to create target branch
# if it exists, check it out instead
git checkout --track origin/${WEBAPPS_BRANCH} || git checkout ${WEBAPPS_BRANCH}
# init webapps.e repo
git submodule update --init --recursive
# set variables based on context from webapps checkout
BRANCH=$(git rev-parse --abbrev-ref HEAD)
COMMIT=$(git rev-parse --short HEAD)
# use the commit message from webapps, qualifying references to webapps PRs to that they
# link to the correct PR from the teleport repo (#123 becomes gravitational/webapps#123)
COMMIT_DESC=$(git log --decorate=off --oneline -1 | sed -E 's.(#[0-9]+).gravitational/webapps\1.g')
COMMIT_URL="https://github.com/gravitational/webapps/commit/${COMMIT}"
AUTO_BRANCH_NAME="webapps-auto-pr-$(date +%s)"

# clone webassets repo (into 'webapps/dist')
git clone git@github.com:gravitational/webassets.git dist
pushd dist; git checkout ${BRANCH} || git checkout -b ${BRANCH}; rm -fr ./*/

# prepare webassets.e repo (in 'webapps/dist/e')
git submodule update --init --recursive
pushd e; git checkout ${BRANCH} || git checkout -b ${BRANCH}; rm -fr ./*/
popd; popd

# build the dist files (in 'webapps')
make build-teleport

# push dist files to webassets/e repoisitory
pushd dist/e
git add -A .
git commit -am "${COMMIT_DESC}" -m "${COMMIT_URL}" --allow-empty
git push origin ${BRANCH}
popd

# push dist files to webassets repository
pushd dist
git add -A .
git commit -am "${COMMIT_DESC}" -m "${COMMIT_URL}" --allow-empty
git push origin ${BRANCH}
popd

# use temporary file to store new webassets commit sha
pushd dist
WEBASSETS_COMMIT_SHA=$(git rev-parse HEAD)
popd

# clone teleport repo
git clone git@github.com:gravitational/teleport.git teleport
pushd teleport

# try to create target branch
# if it exists, check it out instead
git checkout --track origin/${TELEPORT_BRANCH} || git checkout ${TELEPORT_BRANCH}

# update git submodules (webassets/webassets.e)
git fetch --recurse-submodules && git submodule update --init webassets

# check out previously committed SHA
pushd webassets
git checkout ${WEBASSETS_COMMIT_SHA}
popd

# switch to automatic branch and make a commit
git checkout -b ${AUTO_BRANCH_NAME}
git add -A .
git commit -am "[auto] Update webassets in ${TELEPORT_BRANCH}" -m "${COMMIT_DESC} ${COMMIT_URL}" -m "[source: -w ${WEBAPPS_BRANCH}] [target: -t ${TELEPORT_BRANCH}]" --allow-empty
git push --set-upstream origin ${AUTO_BRANCH_NAME}

# run 'gh' to raise a PR to merge this automatic branch into the target branch
gh pr create --base ${TELEPORT_BRANCH} --fill --label automated,webassets
popd

# clean up all the cloned repos
popd
rm -rf $TEMP_DIR
