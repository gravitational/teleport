#!/bin/bash
#
# Ensures local session is logged in with appropriate credentials to 
# successfully execute `make` target "cloud-deploy"

TELEPORT_PROXY=${TELEPORT_PROXY:-platform.teleport.sh}
TELEPORT_USER=${TELEPORT_USER:-$(git config user.email)}
KUBE_TENANT_CLUSTER=${KUBE_TENANT_CLUSTER:-tc-staging-management}
TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
AWS_SSO_PROFILE=${AWS_SSO_PROFILE:-tc-stage-core}
AWS_PROFILE=${AWS_PROFILE:-tc-stage-ecr}

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

export AWS_PROFILE
echo "Checking AWS identity (AWS_PROFILE=\"$AWS_PROFILE\")..."
if aws_sts_ci=$(aws sts get-caller-identity); then
    :
else
    echo "Attempting AWS SSO login using profile \"$AWS_SSO_PROFILE\"..."
    aws sso login --profile=$AWS_SSO_PROFILE
    fail_on_exit_code "Could not login to AWS SSO. Ensure ~/.aws/config contains required profiles (see dev-deploy.md)."
    aws_sts_ci=$(aws sts get-caller-identity)
    fail_on_exit_code "Unexpected error. Could not retrieve AWS STS identity following successful login."
fi

echo "Checking AWS assumed role..."
aws_role_regex="(cloudteam-stage-role|coreteam-localbuilds-ecr-role)"
echo "$aws_sts_ci" | grep -E "$aws_role_regex"
fail_on_exit_code "Current role must contain \"$aws_role_regex\". Please run \"export AWS_PROFILE=tc-stage-ecr\" and retry."

echo "Logging into AWS ECR with docker..."
ecr_token=$(aws ecr get-login-password --region us-west-2)
fail_on_exit_code "Unable to retrieve token from AWS ECR."
docker login --username AWS --password-stdin ${TARGET_IMAGE_REPO} <<< $ecr_token
fail_on_exit_code "Docker login failed."


echo "Logging in to teleport cluster (proxy=$TELEPORT_PROXY, user=$TELEPORT_USER)..."
# check stderr to see if there's an active session
tsh_err=$(tsh status 2>&1 >/dev/null)
if [[ -n $tsh_err ]]; then
    tsh login --proxy=$TELEPORT_PROXY --user $TELEPORT_USER
    fail_on_exit_code "Failed to login to teleport cluster at: $TELEPORT_PROXY"
fi

# check to see if we're connected to the correct proxy
tsh_proxy=$(tsh status | grep $TELEPORT_PROXY)
if [[ -z $tsh_proxy ]]; then
    tsh login --proxy=$TELEPORT_PROXY --user $TELEPORT_USER
    fail_on_exit_code "Failed to login to teleport cluster at: $TELEPORT_PROXY"
fi

echo "Selecting kubernetes cluster \"$KUBE_TENANT_CLUSTER\"..."
tsh kube login $KUBE_TENANT_CLUSTER
fail_on_exit_code "Failed to select kubernetes cluster: $KUBE_TENANT_CLUSTER"

echo "Searching for tenant namespace in k8s cluster..."
ns_prefix="namespace/cloud-gravitational-io-"
ns=$(kubectl get ns --all-namespaces -o name | grep $ns_prefix | head -n 1)
fail_on_exit_code "Unable to find any k8s namespaces having prefix \"$ns_prefix\""
tenant=${ns/$ns_prefix/} # strip namespace prefix
ns=$(cut -d "/" -f 2 <<< $ns) # strip resource kind prefix
echo "Checking for permissions to patch tenant..."
kres=$(kubectl auth can-i patch tenant/$tenant -n $ns) && [[ "${kres}" == "yes" ]]
fail_on_exit_code "Insufficient k8s API permissions on cluster \"$KUBE_TENANT_CLUSTER\" - cannot patch tenant \"$tenant\""

echo_color $green "Success!"
