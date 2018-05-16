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

ABSPATH=$(readlink -f $0)
SCRIPT_DIR=$(dirname $ABSPATH)
BUILD_DIR=$(readlink -f ${SCRIPT_DIR}/build)
YAML_PATH=$(readlink -f ${SCRIPT_DIR}/..)

# Remove existing AMI ID file if present
if [ -f ${BUILD_DIR}/amis.txt ]; then
    rm -f ${BUILD_DIR}/amis.txt
fi

# Read build timestamp from file
TIMESTAMP_FILE=${BUILD_DIR}/${RUN_MODE}_build_timestamp.txt
if [ ! -f ${TIMESTAMP_FILE} ]; then
    echo "Cannot find ${TIMESTAMP_FILE}"
    exit 1
fi
BUILD_TIMESTAMP=$(<${TIMESTAMP_FILE})

# Write AMI ID for each region to AMI ID file
for REGION in ${REGION_LIST}; do
    aws ec2 describe-images --region ${REGION} --filters Name=tag:BuildTimestamp,Values=${BUILD_TIMESTAMP} > ${BUILD_DIR}/${REGION}.json
    AMI_ID=$(jq --raw-output '.Images[0].ImageId' ${BUILD_DIR}/${REGION}.json)
    rm -f ${BUILD_DIR}/${REGION}.json
    echo "${REGION}=${AMI_ID}" >> ${BUILD_DIR}/amis.txt
done

# Update Cloudformation YAML file with new AMI IDs
for REGION in ${REGION_LIST}; do
    CURRENT_AMI_ID=$(grep ${REGION} ${YAML_PATH}/${RUN_MODE}.yaml | awk -F: '{print $3}' | tr -d ' ' | tr -d '}')
    NEW_AMI_ID=$(grep ${REGION} ${BUILD_DIR}/amis.txt | awk -F= '{print $2}')
    if [[ "${NEW_AMI_ID}" == "" || "${NEW_AMI_ID}" == "null" ]]; then
        echo "Error: cannot get AMI ID for ${REGION}"
        exit 2
    elif [[ "${NEW_AMI_ID}" != "${CURRENT_AMI_ID}" ]]; then
        sed -i "s/${CURRENT_AMI_ID}/${NEW_AMI_ID}/g" ${YAML_PATH}/${RUN_MODE}.yaml
        #if [[ "${RUN_MODE}" == "oss" ]]; then # TODO remove
        #    sed -i "s/${CURRENT_AMI_ID}/${NEW_AMI_ID}/g" ${YAML_PATH}/oss-vpc.yaml
        #fi
        echo "AMI ID for ${REGION} changed from ${CURRENT_AMI_ID} to ${NEW_AMI_ID}"
    fi
done
