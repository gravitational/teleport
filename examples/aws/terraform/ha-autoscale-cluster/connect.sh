#!/bin/bash

set -e
# This script can be used to connect to instances behind the SSH bastion. It makes the assumption that either:
# - The required keypair .pem file is in the same directory as this script
# - The required keypair has been added to the running ssh-agent with `ssh-add /path/to/keypair.pem`
# Example usage (server IDs are zero-indexed)
# Connect to the first auth server in the ASG: ./connect.sh auth 0
# Connect to the second proxy server in the ASG: ./connect.sh proxy 1
# Connect to deployed example node: ./connect.sh node
if [[ "$1" != "" ]]; then
    INSTANCE_TYPE="$1"
else
    INSTANCE_TYPE="auth"
fi
if [[ "$2" != "" ]]; then
    INSTANCE_ID="$2"
else
    INSTANCE_ID="0"
fi

BASTION_IP="$(terraform output -raw bastion_ip_public)"
echo "Bastion IP: ${BASTION_IP?}"
CLUSTER_NAME="$(terraform output -raw cluster_name)"
echo "Cluster name: ${CLUSTER_NAME?}"

if [[ "${INSTANCE_TYPE?}" == "auth" ]]; then
    SERVER_IP="$(aws ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${CLUSTER_NAME?}" "Name=tag:TeleportRole,Values=auth" --query "Reservations[${INSTANCE_ID?}].Instances[*].PrivateIpAddress" --output text)"
    echo "Auth ${INSTANCE_ID?} IP: ${SERVER_IP?}"
elif [[ "${INSTANCE_TYPE?}" == "proxy" ]]; then
    SERVER_IP="$(aws ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${CLUSTER_NAME?}" "Name=tag:TeleportRole,Values=proxy" --query "Reservations[${INSTANCE_ID?}].Instances[*].PrivateIpAddress" --output text)"
    echo "Proxy ${INSTANCE_ID?} IP: ${SERVER_IP?}"
elif [[ "${INSTANCE_TYPE?}" == "node" ]]; then
   SERVER_IP="$(aws ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${CLUSTER_NAME?}" "Name=tag:TeleportRole,Values=node" --query "Reservations[*].Instances[*].PrivateIpAddress" --output text)"
    echo "Node IP: ${SERVER_IP?}"
fi

KEYPAIR_NAME="$(terraform output -raw key_name)"
echo "Keypair name: ${KEYPAIR_NAME?}"
ssh -i "${KEYPAIR_NAME?}.pem" -o ProxyCommand="ssh -i \"${KEYPAIR_NAME?}.pem\" -W '[%h]:%p' \"ec2-user@${BASTION_IP?}\"" "ec2-user@${SERVER_IP?}"
