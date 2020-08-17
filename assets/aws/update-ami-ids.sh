#!/bin/bash
set -e

usage() { echo "Usage: $(basename $0) [-m <cloudformation/terraform>] [-t <oss/ent/ent-fips>] [-r <comma-separated regions>] [-v version]" 1>&2; exit 1; }
while getopts ":m:t:r:v:" o; do
    case "${o}" in
        m)
            m=${OPTARG}
            if [[ ${m} != "cloudformation" && ${m} != "terraform" ]]; then usage; fi
            ;;
        r)
            r=${OPTARG}
            ;;
        t)
            t=${OPTARG}
            if [[ ${t} != "oss" && ${t} != "ent" && ${t} != "ent-fips" ]]; then usage; fi
            ;;
         v)
            v=${OPTARG}
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

if [ -z "${m}" ] || [ -z "${r}" ] || [ -z "${t}" ] || [ -z "${v}" ]; then
    usage
fi

MODE=${m}
REGIONS=${r}
TYPE=${t}
VERSION=${v}

# account ID that owns the public images
AWS_ACCOUNT_ID=126027368216

# check that awscli is installed
if [[ ! $(type aws) ]]; then
    echo "aws must be installed"
    exit 1
fi

# check that jq is installed
if [[ ! $(type jq) ]]; then
    echo "jq must be installed"
    exit 2
fi

# get AMI IDs for each region
declare -A IMAGE_IDS
for REGION in ${REGIONS//,/ }; do
    if [[ "${TYPE}" == "ent-fips" ]]; then
        AMI_ID_STUB="gravitational-teleport-ami-ent-${VERSION}-fips"
    else
        AMI_ID_STUB="gravitational-teleport-ami-${TYPE}-${VERSION}"
    fi
    IMAGE_ID=$(aws ec2 describe-images --owners ${AWS_ACCOUNT_ID} --filters "Name=name,Values=${AMI_ID_STUB}" --region ${REGION} | jq -r ".Images[].ImageId")
    if [[ "${IMAGE_ID}" == "" ]]; then
        echo "Error getting ${TYPE} image ID for Teleport ${VERSION} in region ${REGION}"
        exit 3
    fi
    IMAGE_IDS[${REGION}]=${IMAGE_ID}
done

if [[ "${MODE}" == "cloudformation" ]]; then
    if [[ "${TYPE}" == "oss" ]]; then
        CLOUDFORMATION_PATH=../../examples/aws/cloudformation/oss.yaml
    elif [[ "${TYPE}" == "ent" ]]; then
        CLOUDFORMATION_PATH=../../examples/aws/cloudformation/ent.yaml
    elif [[ "${TYPE}" == "ent-fips" ]]; then
        echo "Enterprise FIPS not supported for Cloudformation"
        exit 1
    fi
    # replace AMI ID in place
    for REGION in ${REGIONS//,/ }; do
        OLD_AMI_ID=$(grep $REGION $CLOUDFORMATION_PATH | sed -n -E "s/$REGION: \{HVM64 : (ami.*)\}/\1/p" | tr -d " ")
        NEW_AMI_ID=${IMAGE_IDS[$REGION]}
        sed -i -E "s/$REGION: \{HVM64 : ami(.*)\}$/$REGION: \{HVM64 : $NEW_AMI_ID\}/g" $CLOUDFORMATION_PATH
        echo "[${TYPE}: ${REGION}] ${OLD_AMI_ID} -> ${NEW_AMI_ID}"
    done
elif [[ "${MODE}" == "terraform" ]]; then
    TERRAFORM_PATH=../../examples/aws/terraform/README.md
    if [[ "${TYPE}" == "oss" ]]; then
        TYPE_STRING="OSS"
    elif [[ "${TYPE}" == "ent" ]]; then
        TYPE_STRING="Enterprise"
    elif [[ "${TYPE}" == "ent-fips" ]]; then
        TYPE_STRING="Enterprise FIPS"
    fi
    # replace AMI ID in place
    for REGION in ${REGIONS//,/ }; do
        # ap-south-1 v4.x.x OSS: ami-xxx
        OLD_AMI_ID=$(grep -E "# $REGION v(.*) ${TYPE_STRING}" $TERRAFORM_PATH | sed -n -E "s/# $REGION v(.*) ${TYPE_STRING}: (ami.*)/\2/p" | tr -d " ")
        NEW_AMI_ID=${IMAGE_IDS[$REGION]}
        sed -i -E "s/^# $REGION v(.*) ${TYPE_STRING}: ami(.*)$/# $REGION v${VERSION} ${TYPE_STRING}: $NEW_AMI_ID/g" $TERRAFORM_PATH
        echo "[${TYPE}: ${REGION}] ${OLD_AMI_ID} -> ${NEW_AMI_ID}"
    done
fi