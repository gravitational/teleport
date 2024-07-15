#!/bin/bash

set -euo pipefail

source vars.env

if [[ "$TELEPORT_BACKEND" == "firestore" ]]; then
  exit 0
fi

dynamo_policy="$STATE_DIR/dynamo-iam-policy"

s3_policy="$STATE_DIR/s3-iam-policy"

route53_policy="$STATE_DIR/route53-policy"

if [[ "$TELEPORT_BACKEND" == "dynamo" ]]; then
  aws iam create-policy \
      --policy-name "${CLUSTER_NAME}-dynamo" \
      --policy-document "$(cat "$dynamo_policy")"
fi

aws iam create-policy \
    --policy-name "${CLUSTER_NAME}-s3" \
    --policy-document "$(cat "$s3_policy")"

aws iam create-policy \
    --policy-name "${CLUSTER_NAME}-route53" \
    --policy-document "$(cat "$route53_policy")"

