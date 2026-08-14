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

TENANT=${TENANT:-}
CLOUD_API_APP=${CLOUD_API_APP:-cloud-api-staging}
TC_PATH=${TC_PATH:-../../cloud/tc/cmd/tc}
TELEPORT_HOME=${TELEPORT_HOME:-"$HOME/.tsh_platform"}

echo "-> Deleting tenant \"$TENANT\""
if ! (command -v tc); then
    echo "Using \`tc\` from source in folder \"$TC_PATH\"..."
    tc () {
        cd "$TC_PATH" && go run . "$@"
    }
fi


(TELEPORT_HOME=$TELEPORT_HOME tc tenant delete --app-name="$CLOUD_API_APP" --name="$TENANT")
fail_on_exit_code "Unable to delete tenant \"$TENANT\""
echo "-> Deleted tenant \"$TENANT\""
