cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=auth
EC2_REGION=us-east-1
TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
TELEPORT_CLUSTER_NAME=gus-tftestkube4
TELEPORT_DOMAIN_ADMIN_EMAIL=test@email.com
TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
TELEPORT_DYNAMO_TABLE_NAME=gus-tftestkube4
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=gus-tftestkube4-events
TELEPORT_INFLUXDB_ADDRESS=http://gus-tftestkube4-monitor-ae7983980c3419ab.elb.us-east-1.amazonaws.com:8086
TELEPORT_LICENSE_PATH=/home/gus/downloads/teleport/license-gus.pem
TELEPORT_LOCKS_TABLE_NAME=gus-tftestkube4-locks
TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
USE_ACM=false
EOF
export TELEPORT_TEST_FIPS_MODE=true
