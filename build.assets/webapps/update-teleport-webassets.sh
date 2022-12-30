#!/bin/bash
set -euo pipefail

usage() { echo "Usage: $(basename $0) [-w <webassets branch to update>] [-t <teleport branch to commit to>]" 1>&2; exit 1; }
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

WEBASSETS_BRANCH=${w}
TELEPORT_BRANCH=${t}

# check if gh is installed
if ! type gh >/dev/null 2>&1; then
    echo "The 'gh' utility must be installed to run this script."
    echo "You can download it from https://github.com/cli/cli/releases/latest"
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
if ! git ls-remote --heads --exit-code git@github.com:gravitational/webassets.git ${WEBASSETS_BRANCH}; then
    echo "Cannot find ${WEBASSETS_BRANCH} in the webassets repo."
    exit 1
fi
if ! git ls-remote --heads --exit-code git@github.com:gravitational/teleport.git ${TELEPORT_BRANCH}; then
    echo "Cannot find ${TELEPORT_BRANCH} in the teleport repo."
    exit 1
fi

# keep all the clones in a temp directory
TEMP_DIR="$(mktemp -d)"
pushd $TEMP_DIR

# clone teleport repo
git clone git@github.com:gravitational/teleport.git teleport
pushd teleport

# try to create target branch
# if it exists, check it out instead
git checkout --track origin/${TELEPORT_BRANCH} || git checkout ${TELEPORT_BRANCH}

# update git submodules (webassets/webassets.e)
git fetch --recurse-submodules && git submodule update --init webassets

AUTO_BRANCH_NAME="webassets-auto-pr-$(date +%s)"
git switch -c ${AUTO_BRANCH_NAME}

pushd webassets
git switch ${WEBASSETS_BRANCH}
git pull
popd

git add webassets
git commit -am "[auto] Update webassets in teleport/${TELEPORT_BRANCH} from webassets/${WEBASSETS_BRANCH}" --allow-empty
git push --set-upstream origin ${AUTO_BRANCH_NAME}

# run 'gh' to raise a PR to merge this automatic branch into the target branch
gh pr create --base ${TELEPORT_BRANCH} --fill --label automated,webassets
popd

# clean up all the cloned repos
popd
rm -rf $TEMP_DIR
