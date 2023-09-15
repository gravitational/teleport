write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=agent
EC2_REGION=us-west-2
TELEPORT_AGENT_APP_DESCRIPTION="Production Grafana instance"
TELEPORT_AGENT_APP_ENABLED=true
TELEPORT_AGENT_APP_LABELS="env: prod|app: grafana"
TELEPORT_AGENT_APP_NAME=grafana-prod
TELEPORT_AGENT_APP_URI=grafana001.mycluster.hosting:3000
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
@test "[${TEST_SUITE?}] app_service.enabled is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_BLOCK?}"
    echo "${APP_BLOCK?}" | grep -E "^  enabled: yes"
}

@test "[${TEST_SUITE?}] app_service.apps.name is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_APPS_BLOCK?}"
    echo "${APP_APPS_BLOCK?}" | grep -E "^  - name: ${TELEPORT_AGENT_APP_NAME}"
}

@test "[${TEST_SUITE?}] app_service.apps.description is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_APPS_BLOCK?}"
    echo "${APP_APPS_BLOCK?}" | grep -E "^    description: \"${TELEPORT_AGENT_APP_DESCRIPTION}\""
}

@test "[${TEST_SUITE?}] app_service.apps.uri is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_APPS_BLOCK?}"
    echo "${APP_APPS_BLOCK?}" | grep -E "^    uri: \"${TELEPORT_AGENT_APP_URI}\""
}

@test "[${TEST_SUITE?}] app_service.apps.public_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_APPS_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${APP_APPS_BLOCK?}" | { ! grep -qE "^    public_addr: "; }
}
