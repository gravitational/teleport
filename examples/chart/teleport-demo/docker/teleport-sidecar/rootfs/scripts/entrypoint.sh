#!/bin/bash
set -e
set -x

function entrypoint_log() {
    echo "[entrypoint] $*"
}

# arguments to entrypoint (you can pass any/all of these)
# 'publish-tokens' - generates node and trusted cluster join tokens, then publishes them as kubectl secrets
# if you pass no arguments, the default will run 'publish-tokens'

# handle the case where no arguments are provided
if [[ $# -eq 0 ]]; then
    entrypoint_log "No arguments passed, using defaults"
    PUBLISH_TOKENS="true"
# parse arguments on command line
else
    UNKNOWN=()
    while [[ $# -gt 0 ]]; do
        arg="$1"
        case $arg in
            publish-tokens)
            PUBLISH_TOKENS="true"
            shift
            ;;
            *) # unknown option
            UNKNOWN+=("$1") # save it in an array
            shift
            ;;
        esac
    done
    set -- "${UNKNOWN[@]}" # restore unknown arguments
fi

# display argument state and what we're going to run
for mode in PUBLISH_TOKENS; do
    if [[ "${!mode}" == "true" ]]; then
        entrypoint_log "Will run '${mode}'"
    fi
done

# print any unknown arguments
if [[ ${#UNKNOWN[@]} -gt 0 ]]; then
   for unknown in "${UNKNOWN[@]}"; do
       entrypoint_log "Unknown argument: '${unknown}'"
    done
fi

if [[ "${PUBLISH_TOKENS}" == "true" ]]; then
    entrypoint_log "Running step 'PUBLISH_TOKENS'"
    /usr/bin/teleport-publish-tokens
    echo ""
fi

if [[ "${RUN_TELEPORT}" == "true" ]]; then
    entrypoint_log "Running step 'RUN_TELEPORT'"
    teleport start /etc/teleport/teleport.yaml
    echo ""
fi

entrypoint_log "Done - exiting"
exit 0
