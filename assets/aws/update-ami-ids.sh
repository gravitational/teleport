#!/usr/bin/env bash
set -euo pipefail

# Note: to run this script on MacOS you will need to:
# - install gnu-sed (using Brew), then edit the PATH in your shell's RC file to use the GNU version first
# -- (something like "export PATH=/usr/local/opt/gnu-sed/libexec/gnubin:$PATH")
# - install findutils (using Brew), then edit the PATH in your shell's RC file to use the GNU version first
# -- (something like "export PATH=/usr/local/opt/findutils/libexec/gnubin:$PATH")

# shellcheck disable=SC2086
usage() { echo "Usage: $(basename $0) [-a <AWS account ID>] [-m <cloudformation/terraform>] [-t <oss/ent/ent-fips>] [-r <comma-separated regions>] [-v version]" 1>&2; exit 1; }
while getopts ":a:m:t:r:v:" o; do
    case "${o}" in
        a)
            a=${OPTARG}
            ;;
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

if [ -z "${a}" ] || [ -z "${m}" ] || [ -z "${r}" ] || [ -z "${t}" ] || [ -z "${v}" ]; then
    usage
fi

# account ID that owns the public images
AWS_ACCOUNT_ID=${a}
# mode to run in (either 'cloudformation' or 'terraform')
MODE=${m}
# comma-separated list of regions to get and update AMI IDs for
REGIONS=${r}
# Teleport AMI type (one of 'oss', 'ent' or 'ent-fips')
TYPE=${t}
# Teleport version (without 'v')
VERSION=${v}

# check that awscli is installed
if [[ ! $(type aws) ]]; then
    echo "awscli must be installed"
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
    IMAGE_ID=$(aws ec2 describe-images --owners "${AWS_ACCOUNT_ID}" --filters "Name=name,Values=${AMI_ID_STUB}" "Name=is-public,Values=true" --region "${REGION}" | jq -r ".Images[].ImageId")
    if [[ "${IMAGE_ID}" == "" ]]; then
        echo "Error getting ${TYPE} image ID for Teleport ${VERSION} in region ${REGION}. This can happen if the image has not been made public."
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
        # Enterprise FIPS deployments using Cloudformation are not currently
        # supported, pending an update to the Cloudformation scripts.
        echo "Enterprise FIPS mode is not supported for Cloudformation"
        exit 4
    fi
    # replace AMI ID in place
    for REGION in ${REGIONS//,/ }; do
        OLD_AMI_ID=$(grep "${REGION}" "${CLOUDFORMATION_PATH}" | sed -n -E "s/$REGION: \{HVM64 : (ami.*)\}/\1/p" | tr -d " ")
        NEW_AMI_ID=${IMAGE_IDS[$REGION]}
        sed -i -E "s/$REGION: \{HVM64 : ami(.*)\}$/$REGION: \{HVM64 : $NEW_AMI_ID\}/g" ${CLOUDFORMATION_PATH}
        echo "[${TYPE}: ${REGION}] ${OLD_AMI_ID} -> ${NEW_AMI_ID}"
    done
    # update version number
    sed -i -E "s/# All AMIs from AWS - gravitational-teleport-ami-(.*)/# All AMIs from AWS - gravitational-teleport-ami-${TYPE}-${VERSION}/g" ${CLOUDFORMATION_PATH}
elif [[ "${MODE}" == "terraform" ]]; then
    TERRAFORM_SUBDIR="../../examples/aws/terraform"
    TERRAFORM_PATH="${TERRAFORM_SUBDIR}/AMIS.md"
    # get a list of non-hidden directories one level under the terraform directory (one for each of our different terraform modes)
    pushd ${TERRAFORM_SUBDIR}
    TERRAFORM_MODES="$(find . -mindepth 1 -maxdepth 1 -type d -not -path '*/\.*' -printf '%P\n' | xargs)"
    popd
    if [[ "${TYPE}" == "oss" ]]; then
        TYPE_STRING="OSS"
    elif [[ "${TYPE}" == "ent" ]]; then
        TYPE_STRING="Enterprise"
    elif [[ "${TYPE}" == "ent-fips" ]]; then
        TYPE_STRING="Enterprise FIPS"
    fi
    # change version numbers in TF_VAR_ami_name strings
    # shellcheck disable=SC2086
    for MODE in ${TERRAFORM_MODES}; do
        echo "Updating version in README for ${MODE}"
        sed -i -E "s/gravitational-teleport-ami-${TYPE}-([0-9.]+)/gravitational-teleport-ami-${TYPE}-${VERSION}/g" "${TERRAFORM_SUBDIR}/${MODE}/README.md"
    done
    # replace AMI ID in place
    for REGION in ${REGIONS//,/ }; do
        OLD_AMI_ID=$(grep -E "# $REGION v(.*) ${TYPE_STRING}" $TERRAFORM_PATH | sed -n -E "s/# $REGION v(.*) ${TYPE_STRING}: (ami.*)/\2/p" | tr -d " ")
        NEW_AMI_ID=${IMAGE_IDS[$REGION]}
        sed -i -E "s/^# $REGION v(.*) ${TYPE_STRING}: ami(.*)$/# $REGION v${VERSION} ${TYPE_STRING}: $NEW_AMI_ID/g" ${TERRAFORM_PATH}
        echo "[${TYPE}: ${REGION}] ${OLD_AMI_ID} -> ${NEW_AMI_ID}"
    done
fi
