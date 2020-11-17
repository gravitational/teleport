#!/bin/bash
set -e

# Define list of regions to run in
REGION_LIST="us-east-1 us-east-2 us-west-1 us-west-2 ap-south-1 ap-northeast-2 ap-southeast-1 ap-southeast-2 ap-northeast-1 ca-central-1 eu-central-1 eu-west-1 eu-west-2 sa-east-1"

# Exit if oss/ent parameters not provided
if [[ "$1" == "" ]]; then
    echo "Usage: $(basename $0) [oss/ent/ent-fips]"
    exit 1
else
    RUN_MODE="$1"
fi

ABSPATH=$(readlink -f "$0")
SCRIPT_DIR=$(dirname "${ABSPATH}")
BUILD_DIR=$(readlink -f "${SCRIPT_DIR}/build")

AMI_TAG="production"
OUTFILE="amis.txt"
BUILD_TIMESTAMP_FILENAME="${RUN_MODE}_build_timestamp.txt"
NAME_FILTER="*${RUN_MODE}*"
# Conditionally set variables for FIPS
if [[ "${RUN_MODE}" == "ent-fips" ]]; then
    AMI_TAG="production-fips"
    OUTFILE="amis-fips.txt"
    BUILD_TIMESTAMP_FILENAME="ent_build_timestamp.txt"
    NAME_FILTER="*-fips"
fi

# Remove existing AMI ID file if present
if [ -f "${BUILD_DIR}/${OUTFILE}.txt" ]; then
    rm -f "${BUILD_DIR}/${OUTFILE}.txt"
fi

# Read build timestamp from file
TIMESTAMP_FILE="${BUILD_DIR}/${BUILD_TIMESTAMP_FILENAME}"
if [ ! -f "${TIMESTAMP_FILE}" ]; then
    echo "Cannot find \"${TIMESTAMP_FILE}\""
    exit 1
fi
BUILD_TIMESTAMP=$(<"${TIMESTAMP_FILE}")

# Iterate through AMIs
for REGION in ${REGION_LIST}; do
    AMI_ID=$(aws ec2 describe-images --region ${REGION} --filters "Name=name,Values=${NAME_FILTER}" "Name=tag:BuildTimestamp,Values=${BUILD_TIMESTAMP}" "Name=tag:BuildType,Values=${AMI_TAG}"| jq -r '.Images[0].ImageId')
    if [[ "${AMI_ID}" == "" || "${AMI_ID}" == "null" ]]; then
        echo "Error: cannot get AMI ID for ${REGION}"
        exit 2
    fi
    # Make each AMI public (set launchPermission to 'all')
    aws ec2 modify-image-attribute --region ${REGION} --image-id ${AMI_ID} --launch-permission "Add=[{Group=all}]"
    # Check that the AMI was successfully made public by listing it again
    # The output will be "true" if the AMI is public and "" if it doesn't exist or is private
    PUBLIC_CHECK=$(aws ec2 describe-images --region ${REGION} --filters "Name=image-id,Values=${AMI_ID}" "Name=is-public,Values=true" | jq -r '.Images[].Public')
    if [[ "${PUBLIC_CHECK}" == "true" ]]; then
        echo "AMI ID ${AMI_ID} for ${REGION} set to public"
    else
        echo "WARNING: There was an error making ${AMI_ID} in ${REGION} public!"
    fi
done
