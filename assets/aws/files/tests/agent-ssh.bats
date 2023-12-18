write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=agent
EC2_REGION=us-west-2
TELEPORT_AGENT_SSH_ENABLED=true
TELEPORT_AGENT_SSH_LABELS="env: prod|another: test|third: variable-env"
TELEPORT_JOIN_TOKEN=example-auth-token-for-tests
TELEPORT_PROXY_SERVER_LB=gus-tftestkube4-proxy-bc9ba568645c3d80.elb.us-east-1.amazonaws.com
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

@test "[${TEST_SUITE?}] teleport.proxy_server is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}"
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^  proxy_server:" -A1 | grep -q "${TELEPORT_PROXY_SERVER_LB?}"
}

@test "[${TEST_SUITE?}] teleport.auth_token is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}"
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^  auth_token:" -A1 | grep -q "${TELEPORT_JOIN_TOKEN?}"
}

@test "[${TEST_SUITE?}] auth_service is not enabled" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  enabled: no"
}

@test "[${TEST_SUITE?}] proxy_service is not enabled" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  enabled: no"
}

# in each test, we echo the block so that if the test fails, the block is outputted
@test "[${TEST_SUITE?}] ssh_service.enabled is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${NODE_BLOCK?}"
    echo "${NODE_BLOCK?}" | grep -E "^  enabled: yes"
}

@test "[${TEST_SUITE?}] ssh_service.labels is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${NODE_BLOCK?}"
    # replace | with literal newline (same thing the main script does)
    echo "${NODE_BLOCK?}" | grep -E "^  labels:" | grep -q "${TELEPORT_AGENT_SSH_LABELS//\n/
    ?}"
}
