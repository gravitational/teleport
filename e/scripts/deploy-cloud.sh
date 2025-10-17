#!/bin/bash
#
# Build teleport binaries, create and push a docker image to AWS ECR and
# patch a Teleport Cloud tenant to run the new image..
#

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

# input variables
CURRENT_VERSION=$(perl -n -e'/Version = "(?<version>(?<major>[[:alnum:]]+)\.(?<minor>[[:alnum:]]+)\.(?<patch>[[:alnum:]]+)(?<devtag>-[[:alnum:]\.]+)?)"/ && print "$+{version}"' ../api/version.go)
BASE_IMAGE_REPO=${BASE_IMAGE_REPO:-public.ecr.aws/gravitational/teleport-ent-distroless}
BASE_IMAGE_TAG=${BASE_IMAGE_TAG:-$CURRENT_VERSION}

NAMESPACE_PREFIX=${NAMESPACE_PREFIX:-cloud-gravitational-io}
BUILDDIR=${BUILDDIR:-build}

CURRENT_MAJOR_VERSION=$(echo "${CURRENT_VERSION}" | perl -n -e'/^(?<major>[[:alnum:]]+)./ && print "$+{major}"')
BASE_IMAGE_TAG_MAJOR_VERSION=$(echo "${BASE_IMAGE_TAG}" | perl -n -e'/^(?<major>[[:alnum:]]+)./ && print "$+{major}"')

if [[ "${CURRENT_MAJOR_VERSION}" != "${BASE_IMAGE_TAG_MAJOR_VERSION}" ]]; then
  echo "
!!!!!!!
WARNING: The local repo major version (v${CURRENT_MAJOR_VERSION}) does not match BASE_IMAGE_TAG major version (v${BASE_IMAGE_TAG_MAJOR_VERSION}).
         Automatic upgrades will use 'stable/cloud/v${BASE_IMAGE_TAG_MAJOR_VERSION}' and may not work as expected.
         You should ignore this warning if you know that $(realpath "${BUILDDIR}/teleport") major version is v${BASE_IMAGE_TAG_MAJOR_VERSION}.
!!!!!!!
"
fi

TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
TENANT=${TENANT:-}
CLOUD_SKIP_DEPLOY=${CLOUD_SKIP_DEPLOY:-""}
CLOUD_SKIP_ROLLOUT=${CLOUD_SKIP_ROLLOUT:-""}
CLOUD_SKIP_AGENT_IMMEDIATE_UPDATE=${CLOUD_SKIP_AGENT_IMMEDIATE_UPDATE:-""}
TELEPORT_CLUSTER=${TELEPORT_CLUSTER:-platform.teleport.sh}
KUBE_TENANT_CLUSTER=${KUBE_TENANT_CLUSTER:-tc-staging-management}
KUBE_TENANT_CONTEXT=$TELEPORT_CLUSTER-$KUBE_TENANT_CLUSTER
KUBE_AUTH_CLUSTER=${KUBE_AUTH_CLUSTER:-tc-staging-cs-01-usw2}
KUBE_AUTH_CONTEXT=$TELEPORT_CLUSTER-$KUBE_AUTH_CLUSTER
CLOUD_API_APP=${CLOUD_API_APP:-cloud-api-staging}
TC_PATH=${TC_PATH:-../../cloud/tc/cmd/tc}

if [[ -z "$CLOUD_SKIP_DEPLOY" ]]; then
    [ -z "$TENANT" ] && fail_on_exit_code "Environment variable \"TENANT\" must be set." 1
    echo "-> Checking for tenant \"$TENANT\"..."
    NAMESPACE=${NAMESPACE_PREFIX}-${TENANT}
    kubectl get tenant $TENANT -n $NAMESPACE --context=$KUBE_TENANT_CONTEXT &>/dev/null
    fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\"."
fi

if [[ -n "$RELEASE" ]]; then
    echo "-> Patching tenant to run existing release \"$RELEASE\"..."
    TARGET_IMAGE_REPO=${BASE_IMAGE_REPO}
    target_image_tag=${RELEASE}
else
  # generate an image tag for the target docker image
  commit_short=$(cd .. && git rev-parse --short HEAD)
  timestamp=$(date +"%Y%m%d-%H%M")
  if [[ -z "$TENANT" ]]; then
    git_email=$(git config user.email)
    TENANT=${git_email%%@*}
  fi
  target_image_tag="${BASE_IMAGE_TAG}-${TENANT}-${commit_short}-${timestamp}"
  echo "-> Generated docker image tag \"$target_image_tag\""
  target_image="${TARGET_IMAGE_REPO}:${target_image_tag}"

  base_image="${BASE_IMAGE_REPO}:${BASE_IMAGE_TAG}"
  echo "-> Building docker image by replacing teleport binary in base image \"$base_image\"..."
  # pull base image to local image registry
  echo "-> Retrieving base image \"$base_image\"..."
  docker pull --platform=linux/amd64 "$base_image"
  fail_on_exit_code "Could not retrieve requested base image \"$base_image\". Try setting \"BASE_IMAGE_TAG\" to override the value derived from version.go."

  stdin_dockerfile="FROM $base_image\nCOPY teleport /usr/local/bin/teleport\n"
  # ref: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#pipe-dockerfile-through-stdin
  echo -e "$stdin_dockerfile" | docker build  -t "$target_image" --platform=linux/amd64 --file=- "$BUILDDIR"

  echo "-> Pushing docker image to remote \"$target_image\"..."
  docker push $target_image
  fail_on_exit_code "Failed to push docker image, cannot proceed to patching deployments."
fi

if [[ -n "$CLOUD_SKIP_DEPLOY" ]]; then
  echo "Skipping deployment, tenant has not been updated to run $target_image."
  echo_color $green "Success!"
  exit 0
fi

auth_deployment_generation=$(kubectl get deployment teleport-auth -n $NAMESPACE -o jsonpath='{.status.observedGeneration}' --context=$KUBE_AUTH_CONTEXT)
adg_exit=$?

echo "-> Patching tenant \"$TENANT\" to run image \"$TARGET_IMAGE_REPO:$target_image_tag\"..."
if ! (command -v tc); then
    echo "Using \`tc\` from source in folder \"$TC_PATH\"..."
    tc () {
        cd "$TC_PATH" && go run . "$@"
    }
fi
if (tc tenant get --app-name="$CLOUD_API_APP" --name="$TENANT"); then
    (tc tenant patch set --app-name="$CLOUD_API_APP" --name="$TENANT" --teleport-image-repo="$TARGET_IMAGE_REPO" --teleport-version="$target_image_tag")
    if [[ -z "$CLOUD_SKIP_AGENT_IMMEDIATE_UPDATE" ]]; then
      (tc tenant patch merge --app-name="$CLOUD_API_APP" --name="$TENANT" --json '{"clientServices": [{"type":"Agent", "version":"$target_image_tag", "lastVersion":"$target_image_tag", "updateSchedule":"Immediate"}]}')
    fi
else
	echo "Failed to patch tenant \"$TENANT\" using tc, retrying with kubectl..."
	tenant=$(kubectl get tenant $TENANT --namespace=$NAMESPACE --output=name --context=$KUBE_TENANT_CONTEXT)
	fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\'"
	kubectl patch $tenant -n $NAMESPACE --type merge --patch '{"spec": {"teleportImageRepo": "'"$TARGET_IMAGE_REPO"'", "teleportVersion": "'"$target_image_tag"'"}}' --context=$KUBE_TENANT_CONTEXT
	fail_on_exit_code "Unable to patch tenant \"$TENANT\" in namespace \"$NAMESPACE\""
fi
# warn and exit when tenant has skipReconcile annotation
tenant_json=$(kubectl get tenant $TENANT -n $NAMESPACE -o json --context=$KUBE_TENANT_CONTEXT)
echo "$tenant_json" | jq -r '.metadata.annotations."teleport.sh/skipreconcile"' | grep -v true
fail_on_exit_code "Tenant $TENANT patched successfully. Pod rollout is blocked due to annotation \"teleport.sh/skipreconcile\"."
echo "$tenant_json" | jq -r '.spec.suspended' | grep -v true
fail_on_exit_code "Tenant $TENANT patched successfully. Pod rollout is blocked due to tenant suspension."

if [[ -n "$CLOUD_SKIP_ROLLOUT" ]]; then
	echo "Skipping pod rollout. Use kubectl to check the status of your tenant's pods. (kubectl get pods -n $NAMESPACE --context $KUBE_AUTH_CONTEXT)"
	echo_color $green "Success!"
	exit 0
else
	# can't monitor rollout unless we have the original auth deployment generation
	fail_on_exit_code "Could not retrieve auth deployment generation, cannot monitor rollout." $adg_exit
fi

echo "-> Checking tenant pods in cluster \"$KUBE_AUTH_CLUSTER\"..."
tenant_deployments=$(kubectl get deployments --namespace=$NAMESPACE --selector=app=teleport-cloud,role!=redirect --output=name --context=$KUBE_AUTH_CONTEXT | sort)
deployment_count=$(echo "$tenant_deployments" | wc -l)
echo "-> Found $deployment_count deployments, monitoring rollout..."
auth_deployment=$(echo "$tenant_deployments"| grep auth)
fail_on_exit_code "Could not identify auth deployment, cannot monitor rollout."

# when image changes, the generation of the deployment will change after reconcile
sleep 2
while [[ $auth_deployment_generation == $(kubectl get $auth_deployment -n $NAMESPACE -o jsonpath='{.status.observedGeneration}' --context=$KUBE_AUTH_CONTEXT) ]]; do
	echo "Waiting for auth deployment rollout to start (observedGeneration: $auth_deployment_generation)"
	sleep 2
done

# wait for both auth pods to become ready, then wait for proxy pods
for d in $tenant_deployments; do
	echo "-> Monitoring deployment rollout status of \"$d\"..."
	kubectl rollout status $d -n $NAMESPACE --context=$KUBE_AUTH_CONTEXT
	sleep 2
done

echo_color $green "Success!"
