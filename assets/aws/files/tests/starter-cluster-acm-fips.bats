load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
  [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

# in each test, we echo the block so that if the test fails, we can see the block being tested
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

@test "[${TEST_SUITE?}] auth_service.local_auth is false in FIPS mode" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A2 | grep -q "local_auth: false"
}

@test "[${TEST_SUITE?}] proxy_service.ssh_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  ssh_public_addr:" | grep -q "${TELEPORT_DOMAIN_NAME?}:3023"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_public_addr:" | grep -q "${TELEPORT_DOMAIN_NAME?}:3024"
}

@test "[${TEST_SUITE?}] proxy_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  listen_addr: " | grep -q "0.0.0.0:3023"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_listen_addr: " | grep -q "0.0.0.0:3024"
}

@test "[${TEST_SUITE?}] proxy_service.web_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  web_listen_addr: " | grep -q "0.0.0.0:3080"
}

@test "[${TEST_SUITE?}] proxy_service.kubernetes.public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  kubernetes:" -A3 | grep -E "^    public_addr" | grep -q "['${TELEPORT_DOMAIN_NAME?}:3026']"
}

@test "[${TEST_SUITE?}] node_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${NODE_BLOCK?}"
    echo "${NODE_BLOCK?}" | grep -E "^  listen_addr: " | grep -q "0.0.0.0:3022"
}
