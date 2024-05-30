write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=proxy
EC2_REGION=us-west-2
TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
TELEPORT_CLUSTER_NAME=gus-tftestkube4
TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
TELEPORT_PROXY_SERVER_LB=gus-tftestkube4-proxy-bc9ba568645c3d80.elb.us-east-1.amazonaws.com
TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
TELEPORT_ENABLE_MONGODB=true
TELEPORT_ENABLE_MYSQL=true
TELEPORT_ENABLE_POSTGRES=true
USE_ACM=true
USE_TLS_ROUTING=false
EOF
}

load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
    [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

@test "[${TEST_SUITE?}] config file version is v3" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}"
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^version: v3"
}

@test "[${TEST_SUITE?}] teleport.auth_server is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}"
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^  auth_server: ${TELEPORT_AUTH_SERVER_LB?}"
}

# in each test, we echo the block so that if the test fails, the block is outputted
@test "[${TEST_SUITE?}] proxy_service.public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  public_addr: ${TELEPORT_DOMAIN_NAME?}:443"
}

@test "[${TEST_SUITE?}] proxy_service.ssh_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  ssh_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:3023"
}

@test "[${TEST_SUITE?}] proxy_service.postgres_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  postgres_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:5432"
}

@test "[${TEST_SUITE?}] proxy_service.mysql_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  mysql_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:3036"
}

@test "[${TEST_SUITE?}] proxy_service.mongo_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  mongo_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:27017"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:3024"
}

@test "[${TEST_SUITE?}] proxy_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  listen_addr: 0.0.0.0:3023"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_listen_addr: 0.0.0.0:3024"
}

@test "[${TEST_SUITE?}] proxy_service.web_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  web_listen_addr: 0.0.0.0:3080"
}

@test "[${TEST_SUITE?}] proxy_service.mysql_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  mysql_listen_addr: 0.0.0.0:3036"
}

@test "[${TEST_SUITE?}] proxy_service.postgres_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  postgres_listen_addr: 0.0.0.0:5432"
}

@test "[${TEST_SUITE?}] proxy_service.mongo_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  mongo_listen_addr: 0.0.0.0:27017"
}

@test "[${TEST_SUITE?}] proxy_service.kube_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  kube_public_addr: ${TELEPORT_PROXY_SERVER_LB?}:3026"
}

@test "[${TEST_SUITE?}] proxy_service.kube_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  kube_listen_addr: 0.0.0.0:3026"
}

@test "[${TEST_SUITE?}] proxy_service.https_keypairs is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  https_keypairs:"; }
}

@test "[${TEST_SUITE?}] proxy_service.proxy_protocol is not set to on" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  proxy_protocol: on"; }
}
