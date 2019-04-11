#!/usr/bin/env bash
# This script takes a space-separated list of namespaces which should be deleted by kubectl after the helm chart is gone
set -e

function namespacecleaner_log() {
    echo "[namespace-cleaner] $*"
}

# handle the case where no arguments are provided
if [[ $# -eq 0 ]]; then
    namespacecleaner_log "No namespaces passed as command-line arguments, exiting"
    exit 0
# parse arguments on command line
else
    namespacecleaner_log "Arguments: '$@'"
    while [[ $# -gt 0 ]]; do
        arg="$1"
        namespacecleaner_log "Processing '${arg}'"
        # handle colon separated namespace:secret
        NAMESPACE=${arg}
        kubectl delete namespace ${NAMESPACE} --wait || true
        shift
    done
fi

namespacecleaner_log "Done - exiting"
exit 0