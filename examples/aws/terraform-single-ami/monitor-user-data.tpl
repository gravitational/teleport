#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
TELEPORT_ROLE=monitor
EC2_REGION=${region}
TELEPORT_CLUSTER_NAME=${cluster_name}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_S3_BUCKET=${s3_bucket}
USE_ACM=${use_acm}
EOF