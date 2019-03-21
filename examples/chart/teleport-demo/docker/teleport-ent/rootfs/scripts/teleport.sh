#!/usr/bin/env bash

echo "CLUSTER_TYPE: ${CLUSTER_TYPE}"
echo "ROLE: ${ROLE}"
echo "ARGS: $@"
echo "---"

# copy read only config version to /tmp for editing
cp /etc/teleport/teleport.yaml /tmp/teleport.yaml

# replace node join token in /tmp/teleport.yaml with version from k8s secret
if [[ "${ROLE}" == "node" ]]; then
    /usr/bin/teleport-replace-node-join-token
fi

# start teleport daemon
# run in background to keep executing this script
teleport start -c /tmp/teleport.yaml "$@" &

# add roles to auth servers
if [[ "${ROLE}" == "auth" ]]; then
    ROLES_DONE=false
    while [[ "${ROLES_DONE}" != "true" ]]; do
        /usr/bin/teleport-add-roles
        if [ $? -eq 0 ]; then
            ROLES_DONE=true
        else
            sleep 2
        fi
    done
fi

# add OIDC connector to auth servers
if [[ "${ROLE}" == "auth" ]]; then
    OIDC_DONE=false
    while [[ "${OIDC_DONE}" != "true" ]]; do
        /usr/bin/teleport-add-oidc
        if [ $? -eq 0 ]; then
            OIDC_DONE=true
        else
            sleep 2
        fi
    done
fi

# add trusted cluster YAML
if [[ "${ROLE}" == "auth" ]] && [[ "${CLUSTER_TYPE}" == "secondary" ]]; then
    TRUSTEDCLUSTER_DONE=false
    while [[ "${TRUSTEDCLUSTER_DONE}" != "true" ]]; do
        /usr/bin/teleport-replace-trustedcluster-join-token
        if [ $? -eq 0 ]; then
            /usr/bin/teleport-trustedcluster-join
            if [ $? -eq 0 ]; then
                TRUSTEDCLUSTER_DONE=true
            else
                sleep 5
            fi
        else
            sleep 5
        fi
    done
fi

tail -F /dev/null # keep the container running when teleport isn't the root process