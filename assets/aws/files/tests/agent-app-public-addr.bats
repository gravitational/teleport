write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=agent
EC2_REGION=us-west-2
TELEPORT_AGENT_APP_ENABLED=true
TELEPORT_AGENT_APP_LABELS="env: prod|app: grafana"
TELEPORT_AGENT_APP_NAME=grafana-prod
TELEPORT_AGENT_APP_PUBLIC_ADDR="grafana-prod.teleport.example.com"
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

@test "[${TEST_SUITE?}] app_service.apps.public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${APP_APPS_BLOCK?}"
    echo "${APP_APPS_BLOCK?}" | grep -qE "^    public_addr: \"${TELEPORT_AGENT_APP_PUBLIC_ADDR}\""
}
