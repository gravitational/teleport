cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
export TELEPORT_ROLE=proxy
export EC2_REGION=us-west-2
export TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
export TELEPORT_CLUSTER_NAME=gus-tftestkube4
export TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
export TELEPORT_INFLUXDB_ADDRESS=http://gus-tftestkube4-monitor-ae7983980c3419ab.elb.us-east-1.amazonaws.com:8086
export TELEPORT_PROXY_SERVER_LB=gus-tftestkube4-proxy-bc9ba568645c3d80.elb.us-east-1.amazonaws.com
export TELEPORT_PROXY_SERVER_NLB_ALIAS=""
export TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
export USE_ACM=false
EOF
