#!/bin/bash
#
# Ensures local session is logged in with appropriate credentials to
# successfully execute `make` target "cloud-deploy"

TELEPORT_CLUSTER=${TELEPORT_CLUSTER:-platform.teleport.sh}
TELEPORT_PROXY=${TELEPORT_PROXY:-$TELEPORT_CLUSTER:443}
KUBE_TENANT_CLUSTER=${KUBE_TENANT_CLUSTER:-tc-staging-management}
KUBE_CONTEXT=$TELEPORT_CLUSTER-$KUBE_TENANT_CLUSTER
TELEPORT_USER=${TELEPORT_USER:-$(git config user.email)}
TARGET_IMAGE_REPO=${TARGET_IMAGE_REPO:-599519581022.dkr.ecr.us-west-2.amazonaws.com/teleport-local-build}
AWS_SSO_PROFILE=${AWS_SSO_PROFILE:-tc-stage-core}
AWS_PROFILE=${AWS_PROFILE:-tc-stage-ecr}
CLOUD_API_APP=${CLOUD_API_APP:-cloud-api-staging}
TC_PATH=${TC_PATH:-../../cloud/tc/cmd/tc}
TELEPORT_HOME=${TELEPORT_HOME:-"$HOME/.tsh_platform"}

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

echo "Logging into kube clusters on \"$TELEPORT_PROXY\"..."
TELEPORT_HOME=$TELEPORT_HOME tsh kube login --proxy=$TELEPORT_PROXY --all
fail_on_exit_code "Failed to login to kubernetes cluster on $TELEPORT_PROXY"

echo "Logging into app \"$CLOUD_API_APP\" on \"$TELEPORT_PROXY\"..."
TELEPORT_HOME=$TELEPORT_HOME tsh app login --proxy=$TELEPORT_PROXY $CLOUD_API_APP
fail_on_exit_code "Failed to login to app \"$CLOUD_API_APP\" on $TELEPORT_PROXY"

echo "Checking for tooling to patch tenant..."
if ! (command -v tc); then
    echo "Using \`tc\` from source in folder \"$TC_PATH\"..."
    tc () {
        cd "$TC_PATH" && go run . "$@"
    }
fi
tenant=$(TELEPORT_HOME=$TELEPORT_HOME tc tenant --app-name="$CLOUD_API_APP" list | head -n 1)
fail_on_exit_code "Unable to retrieve tenant from \"$CLOUD_API_APP\". Ensure the \`tc\` executable is available in the path or the source from \"cloud\" repo is found at path \"$TC_PATH\". Override env var \"TC_PATH\" to point to the source folder if necessary. \nRefer to https://github.com/gravitational/teleport.e/blob/master/dev-deploy.md#tc for more information on setting up \`tc\`."

ns="cloud-gravitational-io-$tenant"
echo "Checking for permissions to patch tenants via k8s API... (tenant=\"$tenant\", namespace=\"$ns\")"
kres=$(kubectl auth can-i patch tenant/$tenant -n $ns --context $KUBE_CONTEXT) && [[ "${kres}" == "yes" ]]
fail_on_exit_code "Insufficient k8s API permissions on cluster \"$KUBE_TENANT_CLUSTER\" - cannot patch tenant \"$tenant\""

echo_color $green "Success!"
