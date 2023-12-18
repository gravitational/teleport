write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=agent
EC2_REGION=us-west-2
TELEPORT_AGENT_DB_DESCRIPTION="Production PostgreSQL database"
TELEPORT_AGENT_DB_ENABLED=true
TELEPORT_AGENT_DB_NAME=postgres-production
TELEPORT_AGENT_DB_PROTOCOL=postgres
TELEPORT_AGENT_DB_URI=postgres-prod123.rds.us-west-2.amazonaws.com:5432
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

@test "[${TEST_SUITE?}] db_service.databases.static_labels key does not exist when no labels are set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${DB_DATABASES_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${DB_DATABASES_BLOCK?}" | { ! grep -qE "^    static_labels: "; }
}
