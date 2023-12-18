#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
TELEPORT_ROLE=auth,node,proxy
EC2_REGION=${region}
TELEPORT_AUTH_SERVER_LB=localhost
TELEPORT_AUTH_TYPE=${teleport_auth_type}
TELEPORT_CLUSTER_NAME=${cluster_name}
TELEPORT_DOMAIN_ADMIN_EMAIL=${email}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_EXTERNAL_HOSTNAME=${domain_name}
TELEPORT_DYNAMO_TABLE_NAME=${dynamo_table_name}
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=${dynamo_events_table_name}
TELEPORT_LICENSE_PATH=${license_path}
TELEPORT_LOCKS_TABLE_NAME=${locks_table_name}
TELEPORT_PROXY_SERVER_LB=${domain_name}
TELEPORT_S3_BUCKET=${s3_bucket}
TELEPORT_ENABLE_MONGODB=${enable_mongodb_listener}
TELEPORT_ENABLE_MYSQL=${enable_mysql_listener}
TELEPORT_ENABLE_POSTGRES=${enable_postgres_listener}
# Note: Let's Encrypt will be automatically disabled if using ACM
USE_LETSENCRYPT=${use_letsencrypt}
USE_ACM=${use_acm}
# Note: TLS routing will be automatically enabled by teleport-generate-config when using ACM with the starter-cluster Terraform
USE_TLS_ROUTING=${use_tls_routing}
EOF
