#!/bin/bash

set -euo pipefail

mode="${1:?'mode (one of attach or detach)'}"

case "$mode" in
    attach)
        ;;
    detach)
        ;;
    *)
        echo "ERROR: unknown mode $mode, expected one of 'attach' or 'detach'."
        ;;
esac

source vars.env

if [[ "$TELEPORT_BACKEND" == "firestore" ]]; then
  exit 0
fi

dynamo_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-dynamo"

s3_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-s3"

route53_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-route53"


# used for stripping out node role name
role_arn_prefix="arn:aws:iam::${ACCOUNT_ID}:role/"


log_info() {
    echo "[i] $* [ $(caller | awk '{print $1}') ]" >&2
}

# discover the node groups associated with our eks cluster
nodegroups="$(aws eks list-nodegroups --cluster-name="$CLUSTER_NAME" | jq -r .nodegroups[])"


if test -z "$nodegroups"; then
    log_info "failed to discover node group!"
    exit 1
fi

while read -r ngroup; do
    noderole_arn="$(aws eks describe-nodegroup \
        --cluster-name="$CLUSTER_NAME" \
        --nodegroup-name="$ngroup" \
        | jq -r .nodegroup.nodeRole)"

    if test -z "$noderole_arn"; then
        log_info "failed to discover node role for group '$ngroup'!"
        exit 1
    fi

    noderole_name="${noderole_arn#"$role_arn_prefix"}"

    log_info "${mode}ing policies to discovered node role '$noderole_name'..."

    if [[ "$TELEPORT_BACKEND" == "dynamo" ]]; then
        aws iam "$mode-role-policy" \
            --policy-arn="$dynamo_policy_arn" \
            --role-name="$noderole_name"
    fi

    aws iam "$mode-role-policy" \
        --policy-arn="$s3_policy_arn" \
        --role-name="$noderole_name"

    aws iam "$mode-role-policy" \
        --policy-arn="$route53_policy_arn" \
        --role-name="$noderole_name"

done <<< "$nodegroups"
