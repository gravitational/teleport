load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
    [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

@test "[${TEST_SUITE?}] teleport.auth_servers is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^  auth_servers:" -A1 | grep -q "${TELEPORT_AUTH_SERVER_LB?}"
}

# in each test, we echo the block so that if the test fails, the block is outputted
@test "[${TEST_SUITE?}] ssh_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${NODE_BLOCK?}"
    echo "${NODE_BLOCK?}" | grep -E "^  listen_addr: " | grep -q "0.0.0.0:3022"
}
