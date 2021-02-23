cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=auth,node,proxy
EC2_REGION=us-west-2
TELEPORT_AUTH_SERVER_LB=localhost
TELEPORT_CLUSTER_NAME=gus-startercluster
TELEPORT_DOMAIN_ADMIN_EMAIL=email@example.com
TELEPORT_DOMAIN_NAME=gus-startercluster.gravitational.io
TELEPORT_EXTERNAL_HOSTNAME=gus-startercluster.gravitational.io
TELEPORT_DYNAMO_TABLE_NAME=gus-startercluster
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=gus-startercluster-events
TELEPORT_LICENSE_PATH=/home/gus/downloads/teleport/license-gus.pem
TELEPORT_LOCKS_TABLE_NAME=gus-startercluster-locks
TELEPORT_PROXY_SERVER_LB=gus-startercluster.gravitational.io
TELEPORT_S3_BUCKET=gus-startercluster-s3.gravitational.io
USE_LETSENCRYPT=false
USE_ACM=true
EOF
