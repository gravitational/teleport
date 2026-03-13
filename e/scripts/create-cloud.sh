#!/bin/bash

function echo_color() {
    local color='\033[0;'$1'm'
    local reset='\033[0m'
    echo -en $color
    echo -n "${@:2}"
    echo -e $reset
}
green=32
red=31

function error() {
    echo_color $red "$@"
}

function fail_on_exit_code() {
    local exit_code=${2:-$?}
    local message=${1:-"non-zero exit code on previous command"}

    if [ $exit_code -ne 0 ]; then
        error $message
        error "(exit code: $exit_code)"
        exit $exit_code
    fi
}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${script_dir}/deploy-helper.sh"

CURRENT_VERSION=$(make --no-print-directory -C .. print-version)
BASE_IMAGE_REPO_WAS_SET=${BASE_IMAGE_REPO+x}
BASE_IMAGE_REPO=${BASE_IMAGE_REPO:-public.ecr.aws/gravitational/teleport-ent-distroless}
BASE_IMAGE_TAG_WAS_SET=${BASE_IMAGE_TAG+x}
BASE_IMAGE_TAG=${BASE_IMAGE_TAG:-$CURRENT_VERSION}
TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
TENANT=${TENANT:-}
CLOUD_API_APP=${CLOUD_API_APP:-cloud-api-staging}
TC_PATH=${TC_PATH:-../../cloud/tc/cmd/tc}
BUILDDIR=${BUILDDIR:-build}
REGION=${REGION:-us-west-2}
TELEPORT_HOME=${TELEPORT_HOME:-"$HOME/.tsh_platform"}

if ! (command -v tc); then
    echo "Using \`tc\` from source in folder \"$TC_PATH\"..."
    tc () {
        cd "$TC_PATH" && go run . "$@"
    }
fi


if [[ -n "$RELEASE" ]]; then
    echo "-> Creating tenant \"$TENANT\" to run existing release \"$RELEASE\"..."
    TARGET_IMAGE_REPO=${BASE_IMAGE_REPO}
    target_image_tag=${RELEASE}
else
  echo "-> create-cloud expects a prebuilt linux/amd64 teleport binary at \"${BUILDDIR}/teleport\"."
  echo "-> Build it first with: make build-cloud-teleport-binary"
  ensure_teleport_binary
  resolve_dev_base_image_for_master "$CURRENT_VERSION" "$BASE_IMAGE_TAG_WAS_SET" "$BASE_IMAGE_REPO_WAS_SET" "$BASE_IMAGE_REPO"
  # generate an image tag for the target docker image
  commit_short=$(cd .. && git rev-parse --short HEAD)
  timestamp=$(date +"%Y%m%d-%H%M")
  if [[ -z "$TENANT" ]]; then
    git_email=$(git config user.email)
    TENANT=${git_email%%@*}
  fi
  if [[ "${BASE_IMAGE_TAG}" =~ ^([0-9]+)\.[0-9]+\.[0-9]+-dev-nightly([0-9]{8})-[0-9]+-[0-9a-f]+$ ]]; then
    major="${BASH_REMATCH[1]}"
    nightly_date="${BASH_REMATCH[2]}"
    tenant_short="${TENANT:0:16}"
    target_image_tag="${major}.0.0-nightly${nightly_date}-${tenant_short}-${timestamp}"
  else
    target_image_tag="${BASE_IMAGE_TAG}-${TENANT}-${commit_short}-${timestamp}"
  fi
  echo "-> Generated docker image tag \"$target_image_tag\""
  target_image="${TARGET_IMAGE_REPO}:${target_image_tag}"

  base_image="${BASE_IMAGE_REPO}:${BASE_IMAGE_TAG}"
  echo "-> Building docker image by replacing teleport binary in base image \"$base_image\"..."
  # pull base image to local image registry
  echo "-> Retrieving base image \"$base_image\"..."
  docker pull --platform=linux/amd64 "$base_image"
  fail_on_exit_code "Could not retrieve requested base image \"$base_image\". Try setting both \"BASE_IMAGE_REPO\" and \"BASE_IMAGE_TAG\" to a published image."

  stdin_dockerfile="FROM $base_image\nCOPY teleport /usr/local/bin/teleport\n"
  # ref: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#pipe-dockerfile-through-stdin
  echo -e "$stdin_dockerfile" | docker build  -t "$target_image" --platform=linux/amd64 --file=- "$BUILDDIR"

  echo "-> Pushing docker image to remote \"$target_image\"..."
  docker push $target_image
  fail_on_exit_code "Failed to push docker image, cannot proceed to patching deployments."
fi


(TELEPORT_HOME=$TELEPORT_HOME tc tenant onboard --app-name="$CLOUD_API_APP" --name="$TENANT" --image-repository="$TARGET_IMAGE_REPO" --version="$target_image_tag" --tier="Internal" --region="$REGION")
fail_on_exit_code "Unable to create tenant \"$TENANT\""

# Reduce clientReadySeconds so that future image updates are instantaneous.
(TELEPORT_HOME=$TELEPORT_HOME tc tenant patch merge -f --app-name="$CLOUD_API_APP" --name="$TENANT" --json '{"proxyTunnel":{"clientReadySeconds": 0}}')
fail_on_exit_code "Unable to create tenant \"$TENANT\""

echo "-> Created tenant \"$TENANT\""
