#!/bin/bash
set -euo pipefail

if [[ "$1" != "" ]]; then
    INSTANCE_TYPE=$1
else
    INSTANCE_TYPE="auth"
fi
if [[ "$2" != "" ]]; then
    INSTANCE_ID=$2
else
    INSTANCE_ID="0"
fi

#TF_VAR_region=$(grep region provider.tf | cut -d\" -f2)
#TF_VAR_key_name=$(grep key_name main.tf | cut -d\" -f2)
#TF_VAR_cluster_name=$(grep cluster_name main.tf | cut -d\" -f2)

export BASTION_IP=$(aws --region ${TF_VAR_region} ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name}" "Name=tag:TeleportRole,Values=bastion" --query "Reservations[*].Instances[*].PublicIpAddress" --output text)
echo "Bastion IP: ${BASTION_IP}"

if [[ "${INSTANCE_TYPE}" == "auth" ]]; then
    export SERVER_IP=$(aws --region ${TF_VAR_region} ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name}" "Name=tag:TeleportRole,Values=auth" --query "Reservations[${INSTANCE_ID}].Instances[*].PrivateIpAddress" --output text)
    echo "Auth ${INSTANCE_ID} IP: ${SERVER_IP}"
elif [[ "${INSTANCE_TYPE}" == "proxy" ]]; then
    export SERVER_IP=$(aws --region ${TF_VAR_region} ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name}" "Name=tag:TeleportRole,Values=proxy" --query "Reservations[${INSTANCE_ID}].Instances[*].PrivateIpAddress" --output text)
    echo "Proxy ${INSTANCE_ID} IP: ${SERVER_IP}"
elif [[ "${INSTANCE_TYPE}" == "node" ]]; then
    export SERVER_IP=$(aws --region ${TF_VAR_region} ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name}" "Name=tag:TeleportRole,Values=node" --query "Reservations[*].Instances[*].PrivateIpAddress" --output text)
    echo "Node IP: ${SERVER_IP}"
fi

echo "Keypair name: ${TF_VAR_key_name}"
ssh -J ec2-user@${BASTION_IP} ec2-user@${SERVER_IP}
