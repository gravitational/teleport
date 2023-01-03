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
BASE_DOCKERFILE=${BASE_DOCKERFILE:-"../build.assets/charts/Dockerfile"}
DOCKERFILE_TARGET=${DOCKERFILE_TARGET:-"teleport"}
DEB_PATH=${DEB_PATH:-"teleport.deb"}	# If built by makefile this should be "./teleport-ent_$(VERSION)_$(ARCH).deb"
BASE_IMAGE_REPO=${BASE_IMAGE_REPO:-quay.io/gravitational/teleport-ent}
BASE_IMAGE_TAG=${BASE_IMAGE_TAG:-0.0.1}
NAMESPACE_PREFIX=${NAMESPACE_PREFIX:-cloud-gravitational-io}
BUILDDIR=${BUILDDIR:-build}
TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
TENANT=${TENANT:-}

[ -z "$TENANT" ] && fail_on_exit_code "Environment variable \"TENANT\" must be set." 1
echo "Checking for tenant \"$TENANT\"..."
NAMESPACE=${NAMESPACE_PREFIX}-${TENANT}
kubectl get tenant $TENANT -n $NAMESPACE &>/dev/null
fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\"."

# generate an image tag for the target docker image
commit_short=$(git rev-parse --short HEAD)
timestamp=$(date +"%Y-%m-%d-%H-%M-%S")
target_image_tag="${BASE_IMAGE_TAG}-${TENANT}-${commit_short}-${timestamp}"
echo "Generated docker image tag \"$target_image_tag\""
target_image="${TARGET_IMAGE_REPO}:${target_image_tag}"

if [[ $BASE_DOCKERFILE == "-" ]]; then
	# passing a hyphen in will build image based on an existing image (replace teleport binary only)
	echo "Building docker image based on \"$base_image\"..."
	# pull base image to local image registry
	base_image="${BASE_IMAGE_REPO}:${BASE_IMAGE_TAG}"
	echo "Retrieving base image \"$base_image\"..."
	docker pull --platform=linux/amd64 $base_image
	fail_on_exit_code "Could not find requested base image \"$base_image\""
	stdin_dockerfile="FROM $base_image\nCOPY teleport /usr/local/bin/teleport\n"
	# ref: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#pipe-dockerfile-through-stdin
	echo -e "$stdin_dockerfile" | docker build  -t "$target_image" --platform=linux/amd64 --file=- "$BUILDDIR"
else
	# when dockerfile is provided, build image from the dockerfile
	echo "Building docker image using \"$BASE_DOCKERFILE\"..."
	docker build  -t "$target_image" --target="$DOCKERFILE_TARGET" --platform=linux/amd64 \
		--build-arg DEB_PATH="$DEB_PATH" --file="$BASE_DOCKERFILE" "$BUILDDIR"
fi

echo "Pushing docker image to remote \"$target_image\"..."
docker push $target_image
fail_on_exit_code "Failed to push docker image, cannot proceed to patching deployments."

echo "Patching tenant \"$TENANT\" to run new image..."
tenant=$(kubectl get tenant $TENANT --namespace=$NAMESPACE --output=name)
fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\'"
kubectl patch $tenant -n $NAMESPACE --type merge --patch '{"spec": {"teleportImageRepo": "'"$TARGET_IMAGE_REPO"'", "teleportVersion": "'"$target_image_tag"'"}}'
fail_on_exit_code "Unable to patch tenant \"$TENANT\" in namespace \"$NAMESPACE\""

tenant_deployments=$(kubectl get deployments --namespace=$NAMESPACE --selector=app=teleport-cloud,role!=redirect --output=name | sort)
deployment_count=$(echo "$tenant_deployments" | wc -l)
echo "Found $deployment_count deployments, monitoring rollout..."
auth_deployment=$(echo "$tenant_deployments"| grep auth)
fail_on_exit_code "Could not identify auth deployment, cannot monitor rollout."

# when image changes, tenant operator will scale down to a single auth pod
while [[ $auth_deployment_desired != 1 ]]; do
	sleep 2
	auth_deployment_desired=$(kubectl get $auth_deployment -n $NAMESPACE -o jsonpath='{.status.replicas}')
	echo "Waiting for single pod auth deployment (status.replicas: $auth_deployment_desired)"
done
# once auth instance is ready, tenant operator will scale up to 2 pods
while [[ $auth_deployment_desired != 2 ]]; do
	sleep 2
	auth_deployment_desired=$(kubectl get $auth_deployment -n $NAMESPACE -o jsonpath='{.status.replicas}')
	echo "Waiting for auth deployment to have 2 pods (status.replicas: $auth_deployment_desired)"
done

# wait for both auth pods to become ready, then wait for proxy pods
for d in $tenant_deployments; do
	echo "Monitoring deployment rollout status of \"$d\"..."
	kubectl rollout status $d -n $NAMESPACE
	sleep 2
done

echo_color $green "Success!"
