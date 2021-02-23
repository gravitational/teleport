cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=proxy
EC2_REGION=us-west-2
TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
TELEPORT_CLUSTER_NAME=gus-tftestkube4
TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
TELEPORT_INFLUXDB_ADDRESS=http://gus-tftestkube4-monitor-ae7983980c3419ab.elb.us-east-1.amazonaws.com:8086
TELEPORT_PROXY_SERVER_LB=gus-tftestkube4-proxy-bc9ba568645c3d80.elb.us-east-1.amazonaws.com
TELEPORT_PROXY_SERVER_NLB_ALIAS=gus-tftestkube-nlb.gravitational.io
TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
USE_ACM=true
EOF
