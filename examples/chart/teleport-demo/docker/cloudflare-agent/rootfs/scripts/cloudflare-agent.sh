#!/usr/bin/env bash
set -e
# if you set DEBUG in the kubernetes environment variables, it must be a string "true" rather than a boolean true
if [[ "${DEBUG}" == true ]]; then
    set -x
fi

function cloudflareagent_log() {
    echo "[cloudflare-agent] $*"
}

function process_record() {
    local RUN_MODE="$1"
    local REGISTER_DOMAIN="$2"
    local DNS_RECORD_CONTENT="$3"

    if [[ "${DEBUG}" == "true" ]]; then
        cloudflareagent_log "register_record()"
        cloudflareagent_log "RUN_MODE: ${RUN_MODE}"
        cloudflareagent_log "REGISTER_DOMAIN: ${REGISTER_DOMAIN}"
        cloudflareagent_log "DNS_RECORD_CONTENT: ${DNS_RECORD_CONTENT}"
        cloudflareagent_log "---"
    fi

    if [[ "${REGISTER_DOMAIN}" == "" ]]; then
        cloudflareagent_log "Domain to register not provided, exiting with error"
        exit 4
    fi

    if [[ "${RUN_MODE}" == "create" ]] && [[ "${DNS_RECORD_CONTENT}" == "" ]]; then
            cloudflareagent_log "Running in create mode and record content not provided, exiting with error"
            exit 5
    fi

    # look up zone ID for provided domain
    ZONE_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" -X GET "https://api.cloudflare.com/client/v${API_VERSION}/zones?name=${CLOUDFLARE_DOMAIN}" | jq -r '.result[].id')
    # exit if we can't get it
    if [[ "${ZONE_ID}" == "null" || "${ZONE_ID}" == "" ]]; then
        if [[ "${RUN_MODE}" == "create" ]]; then
            cloudflareagent_log "[create] Couldn't get Cloudflare Zone ID for '${CLOUDFLARE_DOMAIN}' with the provided credentials - exiting with error"
            exit 1
        elif [[ "${RUN_MODE}" == "delete" ]]; then
            cloudflareagent_log "[delete] Couldn't get Cloudflare Zone ID for '${CLOUDFLARE_DOMAIN}' with the provided credentials - exiting"
            return
        fi
    fi

    # look up record ID
    RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" -X GET "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records?name=${REGISTER_DOMAIN}" | jq -r '.result[].id')
    # if it doesn't exist, create/delete a new record
    if [[ "${RECORD_ID}" == "null" || "${RECORD_ID}" == "" ]]; then
        if [[ "${RUN_MODE}" == "create" ]]; then
            cloudflareagent_log "[create] Couldn't get Cloudflare DNS record ID for '${REGISTER_DOMAIN}' within zone '${ZONE_ID}' - creating new record"
            # create record
            CREATED_RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" --data ${DNS_RECORD_CONTENT} -X POST "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records" | jq -r '.result.id')
            # check response
            if [[ "${CREATED_RECORD_ID}" == "null" || "${CREATED_RECORD_ID}" == "" ]]; then
                cloudflareagent_log "Couldn't create Cloudflare DNS record for '${REGISTER_DOMAIN}' under '${ZONE_ID}' - exiting with error"
                exit 2
            else
                cloudflareagent_log "Created Cloudflare DNS record ID '${CREATED_RECORD_ID}' for '${REGISTER_DOMAIN}' under '${ZONE_ID}'"
            fi
        elif [[ "${RUN_MODE}" == "delete" ]]; then
            cloudflareagent_log "[delete] Couldn't get Cloudflare DNS record ID for '${REGISTER_DOMAIN}' within zone '${ZONE_ID}' - exiting"
        fi
    # if it does exist, update/delete the existing record
    else
        if [[ "${RUN_MODE}" == "create" ]]; then
            cloudflareagent_log "[create] Got Cloudflare DNS record ID '${RECORD_ID}' for '${REGISTER_DOMAIN}' - updating record"
            # update record
            UPDATED_RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" --data ${DNS_RECORD_CONTENT} -X PUT "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records/${RECORD_ID}" | jq -r '.result.id')
            # check response
            if [[ "${UPDATED_RECORD_ID}" == "null" || "${UPDATED_RECORD_ID}" == "" ]]; then
                cloudflareagent_log "Couldn't update Cloudflare DNS record for '${REGISTER_DOMAIN}' under '${ZONE_ID}' - exiting with error"
                exit 3
            else
                cloudflareagent_log "Updated Cloudflare DNS record ID '${UPDATED_RECORD_ID}' for '${REGISTER_DOMAIN}' under '${ZONE_ID}'"
            fi
        elif [[ "${RUN_MODE}" == "delete" ]]; then
            cloudflareagent_log "[delete] Got Cloudflare DNS record ID '${RECORD_ID}' for '${REGISTER_DOMAIN}' - deleting record"
            # update record
            UPDATED_RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" -X DELETE "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records/${RECORD_ID}" | jq -r '.result.id')
            # check response
            if [[ "${UPDATED_RECORD_ID}" == "null" || "${UPDATED_RECORD_ID}" == "" ]]; then
                cloudflareagent_log "Couldn't delete Cloudflare DNS record for '${REGISTER_DOMAIN}' under '${ZONE_ID}' - exiting"
            else
                cloudflareagent_log "Deleted Cloudflare DNS record ID '${UPDATED_RECORD_ID}' for '${REGISTER_DOMAIN}' under '${ZONE_ID}'"
            fi
        fi
    fi
}

# default to creation mode if the MODE variable isn't set (to ensure container compatibility with older installations)
if [[ "${MODE}" == "" ]]; then
    MODE="create"
fi

API_KEY=$(cat /etc/cloudflare/api_key)
EMAIL=$(cat /etc/cloudflare/email)
DOMAIN_TO_REGISTER="${CLUSTER_NAME}.${CLOUDFLARE_DOMAIN}"

if [[ "${MODE}" == "create" ]]; then
    TLS_ENABLED=$(cat /etc/teleport-tls/enabled)
    LETSENCRYPT_ENABLED=$(cat /etc/teleport-tls/letsencrypt-enabled)
fi

if [[ "${DEBUG}" == true ]]; then
    cloudflareagent_log "Mode: ${MODE}"
    cloudflareagent_log "Cluster name: ${CLUSTER_NAME}"
    cloudflareagent_log "Cluster type: ${CLUSTER_TYPE}"
    cloudflareagent_log "Service name: ${SERVICE_NAME}"
    cloudflareagent_log "Domain: ${CLOUDFLARE_DOMAIN}"
    cloudflareagent_log "Register: ${DOMAIN_TO_REGISTER}"
    cloudflareagent_log "Cloudflare TTL: ${CLOUDFLARE_TTL}"
    cloudflareagent_log "---"
    cloudflareagent_log "TLS enabled: ${TLS_ENABLED}"
    cloudflareagent_log "Letsencrypt enabled: ${LETSENCRYPT_ENABLED}"
    cloudflareagent_log "Letsencrypt email address: ${LETSENCRYPT_EMAIL}"
    cloudflareagent_log "---"
fi

# if this is the main cluster, we also create a wildcard record so that kubernetes proxy forwarding from Teleport 3.2 will work
WILDCARD_DOMAIN_TO_REGISTER=""
if [[ "${CLUSTER_TYPE}" == "primary" ]]; then
    WILDCARD_DOMAIN_TO_REGISTER="*.${DOMAIN_TO_REGISTER}"
fi

if [[ "${MODE}" == "create" ]]; then
    SERVICE_TYPE=$(kubectl get service ${SERVICE_NAME} -o jsonpath='{.spec.type}')
    if [[ "${SERVICE_TYPE}" != "LoadBalancer" ]]; then
        cloudflareagent_log "Service '${SERVICE_NAME}' is not using 'LoadBalancer', it's using '${SERVICE_TYPE}'"
        cloudflareagent_log "This process doesn't need to run so is exiting with success"
        exit 0
    fi

    EXTERNAL_IP=""
    while [ -z "${EXTERNAL_IP}" ]; do
        cloudflareagent_log "Waiting for external IP address for '${SERVICE_NAME}'..."
        EXTERNAL_IP=$(kubectl get service ${SERVICE_NAME} --template="{{ range .status.loadBalancer.ingress }}{{ .ip }}{{ end }}")
        [ -z "${EXTERNAL_IP}" ] && sleep 10
    done
    cloudflareagent_log "External IP for '${SERVICE_NAME}' is ready"
    cloudflareagent_log "${EXTERNAL_IP}"

    # set TTL if provided - if not, omit it so cloudflare uses auto
    if [[ "${CLOUDFLARE_TTL}" != "" ]]; then
        RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false,\"ttl\":${CLOUDFLARE_TTL}}"
        if [[ "${WILDCARD_DOMAIN_TO_REGISTER}" != "" ]]; then
            WILDCARD_RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${WILDCARD_DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false,\"ttl\":${CLOUDFLARE_TTL}}"
        fi
    else
        RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false}"
        if [[ "${WILDCARD_DOMAIN_TO_REGISTER}" != "" ]]; then
            WILDCARD_RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${WILDCARD_DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false}"
        fi
    fi

    # do registration
    process_record create "${DOMAIN_TO_REGISTER}" "${RECORD_CONTENT}"
    if [[ "${WILDCARD_DOMAIN_TO_REGISTER}" != "" ]]; then
        process_record create "${WILDCARD_DOMAIN_TO_REGISTER}" "${WILDCARD_RECORD_CONTENT}"
    fi

    # run certbot if TLS is enabled and letsencrypt is enabled
    if [[ "${TLS_ENABLED}" == "true" ]] && [[ "${LETSENCRYPT_ENABLED}" == "true" ]]; then
        cloudflareagent_log "TLS/Letsencrypt enabled, running certbot"
        # create certbot.ini file
        cat >/tmp/cloudflare-credentials-certbot.ini <<EOF
dns_cloudflare_email = ${EMAIL}
dns_cloudflare_api_key = ${API_KEY}
EOF
        chmod 600 /tmp/cloudflare-credentials-certbot.ini
        certbot certonly -n --agree-tos --email ${LETSENCRYPT_EMAIL} --dns-cloudflare --dns-cloudflare-credentials /tmp/cloudflare-credentials-certbot.ini -d ${DOMAIN_TO_REGISTER}
        FIRST_TIME=true
    else
        cloudflareagent_log "TLS/Letsencrypt not enabled, exiting"
        exit 0
    fi

    # keep container running in a loop, attempt to renew certificates once a day and then update kubernetes secrets with changed certificates/key
    while true; do
        date
        certbot renew
        kubectl --namespace ${NAMESPACE} create secret generic tls-web --from-file=/etc/letsencrypt/live/${DOMAIN_TO_REGISTER}/fullchain.pem --from-file=/etc/letsencrypt/live/${DOMAIN_TO_REGISTER}/privkey.pem --dry-run -o yaml | kubectl apply -f -
        # wait a day
        sleep 86400
    done
elif [[ "${MODE}" == "delete" ]]; then
    # do deletion
    process_record delete "${DOMAIN_TO_REGISTER}"
    if [[ "${WILDCARD_DOMAIN_TO_REGISTER}" != "" ]]; then
        process_record delete "${WILDCARD_DOMAIN_TO_REGISTER}"
    fi
fi