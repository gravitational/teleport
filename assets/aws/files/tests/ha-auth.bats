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
    echo "${AUTH_BLOCK?}" | grep -E "^  public_addr:" | grep -q "${TELEPORT_AUTH_SERVER_LB?}:3025"
}

@test "[${TEST_SUITE?}] auth_service.cluster_name is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  cluster_name:" | grep -q "${TELEPORT_CLUSTER_NAME?}"
}

@test "[${TEST_SUITE?}] auth_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  listen_addr: " | grep -q "0.0.0.0:3025"
}

@test "[${TEST_SUITE?}] auth_service.second_factor config line is present in non-FIPS mode" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A2 | grep -q "second_factor:"
}
