#!/usr/bin/env bash
set -e
if [[ "${DEBUG}" == true ]]; then
    set -x
fi

function cloudflareagent_log() {
    echo "[cloudflare-agent] $*"
}

API_KEY=$(cat /etc/cloudflare/api_key)
EMAIL=$(cat /etc/cloudflare/email)
DOMAIN_TO_REGISTER="${CLUSTER_NAME}.${CLOUDFLARE_DOMAIN}"

TLS_ENABLED=$(cat /etc/teleport-tls/enabled)
LETSENCRYPT_ENABLED=$(cat /etc/teleport-tls/letsencrypt-enabled)

if [[ "${DEBUG}" == true ]]; then
    cloudflareagent_log "Cloudflare credentials:"
    cloudflareagent_log "API key: ${API_KEY}"
    cloudflareagent_log "Email: ${EMAIL}"
    cloudflareagent_log "----"
    cloudflareagent_log "Cluster name: ${CLUSTER_NAME}"
    cloudflareagent_log "Domain: ${CLOUDFLARE_DOMAIN}"
    cloudflareagent_log "Register: ${DOMAIN_TO_REGISTER}"
    cloudflareagent_log "Cloudflare TTL: ${CLOUDFLARE_TTL}"
    cloudflareagent_log "---"
    cloudflareagent_log "TLS enabled: ${TLS_ENABLED}"
    cloudflareagent_log "Letsencrypt enabled: ${LETSENCRYPT_ENABLED}"
    cloudflareagent_log "Letsencrypt email address: ${LETSENCRYPT_EMAIL}"
    cloudflareagent_log "---"
    cloudflareagent_log "Service name: ${SERVICE_NAME}"
fi

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

# look up zone ID for provided domain
ZONE_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" -X GET "https://api.cloudflare.com/client/v${API_VERSION}/zones?name=${CLOUDFLARE_DOMAIN}" | jq -r '.result[].id')
# exit if we can't get it
if [[ "${ZONE_ID}" == "null" || "${ZONE_ID}" == "" ]]; then
    cloudflareagent_log "Couldn't get Cloudflare Zone ID for '${CLOUDFLARE_DOMAIN}' with the provided credentials. Exiting"
    exit 1
fi

# set TTL if provided - if not, omit it so cloudflare uses auto
if [[ "${CLOUDFLARE_TTL}" != "" ]]; then
    RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false,\"ttl\":${CLOUDFLARE_TTL}}"
else
    RECORD_CONTENT="{\"type\":\"A\",\"name\":\"${DOMAIN_TO_REGISTER}\",\"content\":\"${EXTERNAL_IP}\",\"proxied\":false}"
fi

# look up record ID
RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" -X GET "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records?name=${DOMAIN_TO_REGISTER}" | jq -r '.result[].id')
# if it doesn't exist, create a new record
if [[ "${RECORD_ID}" == "null" || "${RECORD_ID}" == "" ]]; then
    cloudflareagent_log "Couldn't get Cloudflare DNS record ID for '${DOMAIN_TO_REGISTER}' within zone '${ZONE_ID}' - creating new record"
    # create record
    CREATED_RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" --data ${RECORD_CONTENT} -X POST "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records" | jq -r '.result.id')
    # check response
    if [[ "${CREATED_RECORD_ID}" == "null" || "${CREATED_RECORD_ID}" == "" ]]; then
        cloudflareagent_log "Couldn't create Cloudflare DNS record for '${DOMAIN_TO_REGISTER}' under '${ZONE_ID}'. Exiting"
        exit 2
    else
        cloudflareagent_log "Created Cloudflare DNS record '${CREATED_RECORD_ID}' for '${CLOUDFLARE_DOMAIN}' under '${ZONE_ID}'"
    fi
# if it does exist, update the existing record
else
    cloudflareagent_log "Got Cloudflare DNS record ID '${RECORD_ID}' for '${DOMAIN_TO_REGISTER}' - updating record"
    # update record
    UPDATED_RECORD_ID=$(curl -s -H "Content-Type: application/json" -H "X-Auth-Key: ${API_KEY}" -H "X-Auth-Email: ${EMAIL}" --data ${RECORD_CONTENT} -X PUT "https://api.cloudflare.com/client/v${API_VERSION}/zones/${ZONE_ID}/dns_records/${RECORD_ID}" | jq -r '.result.id')
    # check response
    if [[ "${UPDATED_RECORD_ID}" == "null" || "${UPDATED_RECORD_ID}" == "" ]]; then
        cloudflareagent_log "Couldn't update Cloudflare DNS record for '${DOMAIN_TO_REGISTER}' under '${ZONE_ID}'. Exiting"
        exit 3
    else
        cloudflareagent_log "Updated Cloudflare DNS record '${UPDATED_RECORD_ID}' for '${DOMAIN_TO_REGISTER}' under '${ZONE_ID}'"
    fi
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