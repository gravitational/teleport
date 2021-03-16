#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
TELEPORT_ROLE=auth
EC2_REGION=${region}
TELEPORT_AUTH_TYPE=${auth_type}
TELEPORT_AUTH_SERVER_LB=${auth_server_addr}
TELEPORT_CLUSTER_NAME=${cluster_name}
TELEPORT_DOMAIN_ADMIN_EMAIL=${email}
TELEPORT_DOMAIN_NAME=${domain_name}
TELEPORT_DYNAMO_TABLE_NAME=${dynamo_table_name}
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=${dynamo_events_table_name}
TELEPORT_INFLUXDB_ADDRESS=${influxdb_addr}
TELEPORT_LICENSE_PATH=${license_path}
TELEPORT_LOCKS_TABLE_NAME=${locks_table_name}
TELEPORT_S3_BUCKET=${s3_bucket}
USE_ACM=${use_acm}
EOF