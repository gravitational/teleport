write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=auth
EC2_REGION=us-east-1
TELEPORT_AUTH_SERVER_LB=gus-tftestkube4-auth-0f66dd17f8dd9825.elb.us-east-1.amazonaws.com
TELEPORT_CLUSTER_NAME=gus-tftestkube4
TELEPORT_DOMAIN_ADMIN_EMAIL=test@email.com
TELEPORT_DOMAIN_NAME=gus-tftestkube4.gravitational.io
TELEPORT_DYNAMO_TABLE_NAME=gus-tftestkube4
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=gus-tftestkube4-events
TELEPORT_LOCKS_TABLE_NAME=gus-tftestkube4-locks
TELEPORT_S3_BUCKET=gus-tftestkube4.gravitational.io
USE_ACM=false
USE_TLS_ROUTING=false
EOF
}

load fixtures/common

@test "[${TEST_SUITE?}] config file was generated without error" {
    [ ${GENERATE_EXIT_CODE?} -eq 0 ]
}

# in each test, we echo the block so that if the test fails, the block is outputted
@test "[${TEST_SUITE?}] auth_service.license_file is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${AUTH_BLOCK?}" | { ! grep -qE "^  license_file: "; }
}
