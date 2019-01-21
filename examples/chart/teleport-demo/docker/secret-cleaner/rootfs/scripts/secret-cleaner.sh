#!/bin/bash
# This script takes a space-separated list of secrets which should be deleted by kubectl before other containers are
# started. It is used to clean up any lingering secrets before new Teleport clusters are created by helm and avoid
# issues with out of date join tokens.
# If a name like "secret1" is provided, it will be deleted from the default namespace.
# if a name like "secretnamespace:secret2" is provided, 'secret2' will be deleted from 'secretnamespace'
set -e

function secretcleaner_log() {
    echo "[secret-cleaner] $*"
}

# handle the case where no arguments are provided
if [[ $# -eq 0 ]]; then
    secretcleaner_log "No secret names passed as command-line arguments, exiting"
    exit 0
# parse arguments on command line
else
    secretcleaner_log "Arguments: '$@'"
    while [[ $# -gt 0 ]]; do
        arg="$1"
        secretcleaner_log "Processing '${arg}'"
        # handle colon separated namespace:secret
        if [[ "$arg" =~ ":" ]]; then
            NAMESPACE=$(echo "${arg}" | cut -d: -f1)
            SECRETNAME=$(echo "${arg}" | cut -d: -f2)
            # || true is needed to avoid the script exiting if secret deletion fails due to 'set -e'
            # secret deletion is optional cleanup, this job shouldn't fail if the secret doesn't exist
            kubectl delete --namespace ${NAMESPACE} secrets ${SECRETNAME} || true
        # regular deletion
        else
            SECRETNAME=${arg}
            kubectl delete secrets ${SECRETNAME} || true
        fi
        shift
    done
fi

secretcleaner_log "Done - exiting"
exit 0