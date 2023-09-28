#!/bin/bash

set -euo pipefail

source vars.env

if [[ "$TELEPORT_BACKEND" != "dynamo" ]]; then
    # non-dynamo backeds do not currently use external
    # storage resources
    exit 0
fi

# delete dynamo tables

aws dynamodb delete-table \
    --table-name "${CLUSTER_NAME}-backend"

aws dynamodb delete-table \
    --table-name "${CLUSTER_NAME}-events"

# empty the session bucket

contents="$(aws s3 ls "$SESSION_BUCKET")"

if ! test -z "$contents"; then
    aws s3api delete-objects \
      --bucket "$SESSION_BUCKET" \
      --delete "$( \
        aws s3api list-object-versions \
        --bucket "$SESSION_BUCKET" \
        --output=json \
        --query='{Objects: Versions[].{Key:Key,VersionId:VersionId}}' \
        )"
fi

# delete the session bucket

aws s3api delete-bucket \
    --bucket "$SESSION_BUCKET"
