write_confd_file() {
    cat << EOF > ${TELEPORT_CONFD_DIR?}/conf
TELEPORT_ROLE=auth,node,proxy
EC2_REGION=us-west-2
TELEPORT_AUTH_TYPE=saml
TELEPORT_AUTH_SERVER_LB=localhost
TELEPORT_CLUSTER_NAME=gus-startercluster
TELEPORT_DOMAIN_ADMIN_EMAIL=email@example.com
TELEPORT_DOMAIN_NAME=gus-startercluster.gravitational.io
TELEPORT_EXTERNAL_HOSTNAME=gus-startercluster.gravitational.io
TELEPORT_DYNAMO_TABLE_NAME=gus-startercluster
TELEPORT_DYNAMO_EVENTS_TABLE_NAME=gus-startercluster-events
TELEPORT_LICENSE_PATH=/home/gus/downloads/teleport/license-gus.pem
TELEPORT_LOCKS_TABLE_NAME=gus-startercluster-locks
TELEPORT_PROXY_SERVER_LB=gus-startercluster.gravitational.io
TELEPORT_S3_BUCKET=gus-startercluster-s3.gravitational.io
TELEPORT_ENABLE_MONGODB=false
TELEPORT_ENABLE_MYSQL=false
TELEPORT_ENABLE_POSTGRES=false
USE_LETSENCRYPT=true
USE_ACM=false
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

@test "[${TEST_SUITE?}] auth_service.authnetication.second_factor config line is present" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A3 | grep -q "second_factor:"
}

@test "[${TEST_SUITE?}] auth_service.license_file is set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  license_file: "
}

@test "[${TEST_SUITE?}] auth_service.authentication.type is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^    type: saml"
}

@test "[${TEST_SUITE?}] auth_service.authentication.webauthn.rp_id is set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${AUTH_BLOCK?}"
    echo "${AUTH_BLOCK?}" | grep -E "^  authentication:" -A5 | grep -q "rp_id: ${TELEPORT_DOMAIN_NAME?}"
}

@test "[${TEST_SUITE?}] proxy_service.ssh_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  ssh_public_addr: ${TELEPORT_DOMAIN_NAME?}:3023"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_public_addr: ${TELEPORT_DOMAIN_NAME?}:3024"
}

@test "[${TEST_SUITE?}] proxy_service.postgres_public_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  postgres_public_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.mongo_public_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  mongo_public_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.mysql_public_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  mysql_public_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  listen_addr: 0.0.0.0:3023"
}

@test "[${TEST_SUITE?}] proxy_service.tunnel_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  tunnel_listen_addr: 0.0.0.0:3024"
}

@test "[${TEST_SUITE?}] proxy_service.web_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  web_listen_addr: 0.0.0.0:443"
}

@test "[${TEST_SUITE?}] proxy_service.postgres_listen_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  postgres_listen_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.mongo_listen_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  mongo_listen_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.mysql_listen_addr is not set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  mysql_listen_addr: "; }
}

@test "[${TEST_SUITE?}] proxy_service.kube_public_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  kube_public_addr: ${TELEPORT_DOMAIN_NAME?}:3026"
}

@test "[${TEST_SUITE?}] proxy_service.kube_listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  kube_listen_addr: 0.0.0.0:3026"
}

@test "[${TEST_SUITE?}] proxy_service.https_keypairs is set" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    echo "${PROXY_BLOCK?}" | grep -E "^  https_keypairs:"
}

@test "[${TEST_SUITE?}] node_service.listen_addr is set correctly" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${NODE_BLOCK?}"
    echo "${NODE_BLOCK?}" | grep -E "^  listen_addr: 0.0.0.0:3022"
}

@test "[${TEST_SUITE?}] proxy_service.proxy_protocol is not set to on" {
    load ${TELEPORT_CONFD_DIR?}/conf
    echo "${PROXY_BLOCK?}"
    # this test inverts the regular behaviour of grep -q, so only succeeds if the line _isn't_ present
    echo "${PROXY_BLOCK?}" | { ! grep -qE "^  proxy_protocol: on"; }
}
