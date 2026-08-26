#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
TELEPORT_ROLE=proxy
EC2_REGION=${region}
TELEPORT_AUTH_SERVER_LB=${auth_server_addr}
TELEPORT_CLUSTER_NAME=${cluster_name}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_PROXY_SERVER_LB=${proxy_server_lb_addr}
TELEPORT_PROXY_SERVER_NLB_ALIAS=${proxy_server_nlb_alias}
TELEPORT_S3_BUCKET=${s3_bucket}
TELEPORT_ENABLE_MONGODB=${enable_mongodb_listener}
TELEPORT_ENABLE_MYSQL=${enable_mysql_listener}
TELEPORT_ENABLE_POSTGRES=${enable_postgres_listener}
USE_ACM=${use_acm}
USE_TLS_ROUTING=${use_tls_routing}
EOF
cat >>/etc/default/teleport <<EOF
EC2_REGION=${region}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_S3_BUCKET=${s3_bucket}
EOF
