write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=auth
EC2_REGION=us-east-1
TELEPORT_AUTH_TYPE=github
TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
TELEPORT_CLUSTER_NAME=gus-tftestkube4
TELEPORT_DOMAIN_ADMIN_EMAIL=test@email.com
TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
TELEPORT_DYNAMO_TABLE_NAME=gus-tftestkube4
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=gus-tftestkube4-events
TELEPORT_LICENSE_PATH=/home/gus/downloads/teleport/license-gus.pem
TELEPORT_LOCKS_TABLE_NAME=gus-tftestkube4-locks
TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
USE_ACM=false
USE_TLS_ROUTING=true
EOF
}

load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
    [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

# in each test, we echo the block so that if the test fails, the block is outputted
@test "[${TEST_SUITE?}] teleport.storage.type is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${TELEPORT_BLOCK?}"
    echo "${TELEPORT_BLOCK?}" | grep -E "^    type: dynamodb"
}

@test "[${TEST_SUITE?}] teleport.storage.region is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${TELEPORT_BLOCK?}"
    echo "${TELEPORT_BLOCK?}" | grep -E "^    region: ${EC2_REGION?}"
}

@test "[${TEST_SUITE?}] teleport.storage.table_name is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${TELEPORT_BLOCK?}"
    echo "${TELEPORT_BLOCK?}" | grep -E "^    table_name: ${TELEPORT_DYNAMO_TABLE_NAME?}"
}

@test "[${TEST_SUITE?}] teleport.storage.audit_events_uri is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${TELEPORT_BLOCK?}"
    echo "${TELEPORT_BLOCK?}" | grep -E "^    audit_events_uri: dynamodb://${TELEPORT_DYNAMO_EVENTS_TABLE_NAME?}"
}

@test "[${TEST_SUITE?}] auth_service.public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  public_addr: ${TELEPORT_AUTH_SERVER_LB?}:3025"
}

@test "[${TEST_SUITE?}] auth_service.cluster_name is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  cluster_name: ${TELEPORT_CLUSTER_NAME?}"
}

@test "[${TEST_SUITE?}] auth_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  listen_addr: 0.0.0.0:3025"
}

@test "[${TEST_SUITE?}] auth_service.authentication.second_factor config line is present" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A3 | grep -q "second_factor:"
}

@test "[${TEST_SUITE?}] auth_service.authentication.webauthn.rp_id is set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A5 | grep -q "rp_id: ${TELEPORT_DOMAIN_NAME?}"
}

@test "[${TEST_SUITE?}] auth_service.license_file is set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  license_file: "
}

@test "[${TEST_SUITE?}] auth_service.authentication.type is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^    type: github"
}

@test "[${TEST_SUITE?}] auth_service.proxy_listener_mode is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  proxy_listener_mode: multiplex"
}
