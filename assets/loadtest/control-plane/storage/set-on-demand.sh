#!/bin/bash

set -euo pipefail

source vars.env

# update billing mode of tables

aws dynamodb update-table \
    --table-name "${CLUSTER_NAME}-backend" \
    --billing-mode "PAY_PER_REQUEST" \
    > /dev/null

aws dynamodb update-table \
    --table-name "${CLUSTER_NAME}-events" \
    --billing-mode "PAY_PER_REQUEST" \
    > /dev/null
