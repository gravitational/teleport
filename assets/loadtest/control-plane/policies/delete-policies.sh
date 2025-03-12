#!/bin/bash

set -euo pipefail

source vars.env

if [[ "$TELEPORT_BACKEND" == "firestore" ]]; then
  exit 0
fi

dynamo_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-dynamo"

s3_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-s3"

route53_policy_arn="arn:aws:iam::${ACCOUNT_ID}:policy/${CLUSTER_NAME}-route53"

if [[ "$TELEPORT_BACKEND" == "dynamo" ]]; then
    aws iam delete-policy \
        --policy-arn "$dynamo_policy_arn"
fi

aws iam delete-policy \
    --policy-arn "$s3_policy_arn"

aws iam delete-policy \
    --policy-arn "$route53_policy_arn"
