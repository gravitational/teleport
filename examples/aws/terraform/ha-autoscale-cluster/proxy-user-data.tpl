#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
TELEPORT_ROLE=proxy
EC2_REGION=${region}
TELEPORT_AUTH_SERVER_LB=${auth_server_addr}
TELEPORT_CLUSTER_NAME=${cluster_name}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_INFLUXDB_ADDRESS=${influxdb_addr}
TELEPORT_PROXY_SERVER_LB=${proxy_server_lb_addr}
TELEPORT_PROXY_SERVER_NLB_ALIAS=${proxy_server_nlb_alias}
TELEPORT_S3_BUCKET=${s3_bucket}
USE_ACM=${use_acm}
EOF