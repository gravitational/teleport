#!/bin/bash
set -e

# Define list of regions to run in
REGION_LIST="eu-west-1 us-east-1 us-east-2 us-west-2"

# Exit if oss/ent parameters not provided
if [[ "$1" == "" ]]; then
    echo "Usage: $(basename $0) [oss/ent]"
    exit 1
else
    RUN_MODE="$1"
fi

ABSPATH=$(readlink -f "$0")
SCRIPT_DIR=$(dirname "${ABSPATH}")
BUILD_DIR=$(readlink -f "${SCRIPT_DIR}/build")

# Remove existing AMI ID file if present
if [ -f "${BUILD_DIR}/amis.txt" ]; then
    rm -f "${BUILD_DIR}/amis.txt"
fi

# Read build timestamp from file
TIMESTAMP_FILE="${BUILD_DIR}/${RUN_MODE}_build_timestamp.txt"
if [ ! -f "${TIMESTAMP_FILE}" ]; then
    echo 'Cannot find "${TIMESTAMP_FILE}"'
    exit 1
fi
BUILD_TIMESTAMP=$(<"${TIMESTAMP_FILE}")

# Write AMI ID for each region to AMI ID file
for REGION in ${REGION_LIST}; do
    aws ec2 describe-images --region ${REGION} --filters Name=tag:BuildTimestamp,Values=${BUILD_TIMESTAMP},Name=tag:BuildType,Values=production > "${BUILD_DIR}/${REGION}.json"
    AMI_ID=$(jq --raw-output '.Images[0].ImageId' "${BUILD_DIR}/${REGION}.json")
    if [[ "${AMI_ID}" == "" || "${AMI_ID}" == "null" ]]; then
        echo "Error: cannot get AMI ID for ${REGION}"
        exit 2
    fi
    rm -f "${BUILD_DIR}/${REGION}.json"
    echo "${REGION}=${AMI_ID}" >> "${BUILD_DIR}/amis.txt"
done

# Make each AMI public (set launchPermission to 'all')
for REGION in ${REGION_LIST}; do
    AMI_ID=$(grep ${REGION} "${BUILD_DIR}/amis.txt" | awk -F= '{print $2}')
    if [[ "${AMI_ID}" == "" || "${AMI_ID}" == "null" ]]; then
        echo "Error: cannot get AMI ID for ${REGION}"
        exit 3
    else
        aws ec2 modify-image-attribute --region ${REGION} --image-id ${AMI_ID} --launch-permission "Add=[{Group=all}]"
        echo "AMI ID ${AMI_ID} for ${REGION} set to public"
    fi
done
