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
BASE_IMAGE_REPO=${BASE_IMAGE_REPO:-public.ecr.aws/gravitational/teleport-ent}
BASE_IMAGE_TAG=${BASE_IMAGE_TAG:-$(perl -n -e'/Version = "([[:alnum:]\.]*)[-"]/ && print $1' ../version.go)}
NAMESPACE_PREFIX=${NAMESPACE_PREFIX:-cloud-gravitational-io}
BUILDDIR=${BUILDDIR:-build}
TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
TENANT=${TENANT:-}
CLOUD_SKIP_DEPLOY=${CLOUD_SKIP_DEPLOY:-""}
CLOUD_SKIP_ROLLOUT=${CLOUD_SKIP_ROLLOUT:-""}
KUBE_TENANT_CLUSTER=${KUBE_TENANT_CLUSTER:-tc-staging-management}
KUBE_AUTH_CLUSTER=${KUBE_AUTH_CLUSTER:-tc-staging-cs-01-usw2}

[ -z "$TENANT" ] && fail_on_exit_code "Environment variable \"TENANT\" must be set." 1
echo "-> Checking for tenant \"$TENANT\"..."
tsh kube login "$KUBE_TENANT_CLUSTER"
NAMESPACE=${NAMESPACE_PREFIX}-${TENANT}
kubectl get tenant $TENANT -n $NAMESPACE &>/dev/null
fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\"."

# generate an image tag for the target docker image
commit_short=$(cd .. && git rev-parse --short HEAD)
timestamp=$(date +"%Y%m%d-%H%M")
target_image_tag="${BASE_IMAGE_TAG}-${TENANT}-${commit_short}-${timestamp}"
echo "-> Generated docker image tag \"$target_image_tag\""
target_image="${TARGET_IMAGE_REPO}:${target_image_tag}"

base_image="${BASE_IMAGE_REPO}:${BASE_IMAGE_TAG}"
echo "-> Building docker image by replacing teleport binary in base image \"$base_image\"..."
# pull base image to local image registry
echo "-> Retrieving base image \"$base_image\"..."
docker pull --platform=linux/amd64 "$base_image"
fail_on_exit_code "Could not retrieve requested base image \"$base_image\". Try setting \"BASE_IMAGE_TAG\" to override the value derived from version.go."

# check glibc version before proceeding
echo "-> Checking teleport binary for compatibility with base image..."
base_glibc=$(docker run --entrypoint="/usr/bin/ldd" $base_image --version | head -n1 | sed 's/.*GLIBC \([.0-9]*\).*/\1/g')
binary_glibc=$(objdump -T $BUILDDIR/teleport | grep GLIBC | sed 's/.*GLIBC_\([.0-9]*\).*/\1/g' | sort -ruV | head -n1)
echo "Base image glibc: $base_glibc, binary glibc: $binary_glibc"
echo "$binary_glibc $base_glibc" | awk '{exit !($1 <= $2)}'
fail_on_exit_code "Base image supports glibc version up to $base_glibc. Binary \"$BUILDDIR/teleport\" requires glibc version $binary_glibc. Suggest producing binaries via dockerized build and running \"make CLOUD_SKIP_BUILD=1 deploy-cloud\""

stdin_dockerfile="FROM $base_image\nCOPY teleport /usr/local/bin/teleport\n"
# ref: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#pipe-dockerfile-through-stdin
echo -e "$stdin_dockerfile" | docker build  -t "$target_image" --platform=linux/amd64 --file=- "$BUILDDIR"

echo "-> Pushing docker image to remote \"$target_image\"..."
docker push $target_image
fail_on_exit_code "Failed to push docker image, cannot proceed to patching deployments."

if [[ -n "$CLOUD_SKIP_DEPLOY" ]]; then
	echo "Skipping deployment, tenant has not been updated to run $target_image."
	echo_color $green "Success!"
	exit 0
fi

echo "-> Patching tenant \"$TENANT\" to run new image..."
tsh kube login $KUBE_TENANT_CLUSTER
tenant=$(kubectl get tenant $TENANT --namespace=$NAMESPACE --output=name)
fail_on_exit_code "Tenant \"$TENANT\" not found in namespace \"$NAMESPACE\'"
kubectl patch $tenant -n $NAMESPACE --type merge --patch '{"spec": {"teleportImageRepo": "'"$TARGET_IMAGE_REPO"'", "teleportVersion": "'"$target_image_tag"'"}}'
fail_on_exit_code "Unable to patch tenant \"$TENANT\" in namespace \"$NAMESPACE\""

# warn and exit when tenant has skipReconcile annotation
tenant_json=$(kubectl get tenant $TENANT -n $NAMESPACE -o json)
echo "$tenant_json" | jq -r '.metadata.annotations."teleport.sh/skipreconcile"' | grep -v true
fail_on_exit_code "Tenant $TENANT patched successfully. Pod rollout is blocked due to annotation \"teleport.sh/skipreconcile\"."
echo "$tenant_json" | jq -r '.spec.suspended' | grep -v true
fail_on_exit_code "Tenant $TENANT patched successfully. Pod rollout is blocked due to tenant supension."

if [[ -n "$CLOUD_SKIP_ROLLOUT" ]]; then
	echo "Skipping pod rollout. Use kubectl to check the status of your tenant's pods. (kubectl get pods -n $NAMESPACE)"
	echo_color $green "Success!"
	exit 0
fi

echo "-> Checking tenant pods in cluster \"$KUBE_AUTH_CLUSTER\"..."
tsh kube login $KUBE_AUTH_CLUSTER
tenant_deployments=$(kubectl get deployments --namespace=$NAMESPACE --selector=app=teleport-cloud,role!=redirect --output=name | sort)
deployment_count=$(echo "$tenant_deployments" | wc -l)
echo "-> Found $deployment_count deployments, monitoring rollout..."
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
	echo "-> Monitoring deployment rollout status of \"$d\"..."
	kubectl rollout status $d -n $NAMESPACE
	sleep 2
done

echo_color $green "Success!"
