write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=agent
EC2_REGION=us-west-2
TELEPORT_AGENT_KUBE_CLUSTER_NAME=kube-cluster-name
TELEPORT_AGENT_KUBE_ENABLED=true
TELEPORT_AGENT_KUBE_LABELS="env: prod|another: test|third: variable-env"
TELEPORT_JOIN_TOKEN=example-auth-token-for-tests
TELEPORT_PROXY_SERVER_LB=gus-tftestkube4-proxy-bc9ba568645c3d80.elb.us-east-1.amazonaws.com
EOF
}

load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
    [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

@test "[${TEST_SUITE?}] teleport.auth_servers is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    cat "${TELEPORT_CONFIG_PATH?}"
    cat "${TELEPORT_CONFIG_PATH?}" | grep -E "^  auth_servers:" -A1 | grep -q "${TELEPORT_PROXY_SERVER_LB?}"
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
@test "[${TEST_SUITE?}] kubernetes_service.enabled is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${KUBE_BLOCK?}"
    echo "${KUBE_BLOCK?}" | grep -E "^  enabled: yes"
}

@test "[${TEST_SUITE?}] kubernetes_service.kube_cluster_name is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${KUBE_BLOCK?}"
    echo "${KUBE_BLOCK?}" | grep -E "^  kube_cluster_name: ${TELEPORT_AGENT_KUBE_CLUSTER_NAME}"
}

@test "[${TEST_SUITE?}] kubernetes_service.labels is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${KUBE_BLOCK?}"
    # replace | with literal newline (same thing the main script does)
    echo "${KUBE_BLOCK?}" | grep -E "^  labels:" | grep -q "${TELEPORT_AGENT_KUBE_LABELS//\n/
    ?}"
}
