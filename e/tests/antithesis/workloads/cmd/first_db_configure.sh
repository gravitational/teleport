#!/usr/bin/env bash
set -euo pipefail

PROXY_ADDR="${PROXY_ADDR:-antithesis.teleport.local:3080}"
BOT_IDENTITY="${BOT_IDENTITY:-/creds/admin/identity}"
CA_FILE="${TEST_PG_CERT_DIR:-/testcerts/pg}/ca.crt"
DB_CA_FILE="${TEST_PG_DB_CA_FILE:-/certs/rootCA.pem}"
PG_HOST="${TEST_PG_HOST:-pgtest}"
DYNAMIC_DB_NAME="${TEST_DYNAMIC_DB_NAME:-dynamic}"

if [[ ! -s "$BOT_IDENTITY" ]]; then
    echo "identity file not found or empty: $BOT_IDENTITY" >&2
    exit 1
fi

if [[ ! -s "$DB_CA_FILE" ]]; then
    echo "database CA file not found or empty: $DB_CA_FILE" >&2
    exit 1
fi

echo "exporting Teleport db-client CA via ${PROXY_ADDR}..."
new_ca="$(tctl --auth-server="$PROXY_ADDR" --identity="$BOT_IDENTITY" auth export --type=db-client)"

if ! grep -qF "$new_ca" "$CA_FILE" 2>/dev/null; then
    echo "$new_ca" >>"$CA_FILE"
fi

echo "reloading ${PG_HOST} to pick up the updated trust store..."
PGPASSWORD="${TEST_PG_ADMIN_PASSWORD:-postgres}" psql \
    "host=${PG_HOST} port=5432 dbname=postgres user=postgres sslmode=require" \
    -c "SELECT pg_reload_conf();"

echo "registering dynamic Teleport database ${DYNAMIC_DB_NAME} for ${PG_HOST}:5432..."
db_ca_cert="$(sed 's/^/      /' "$DB_CA_FILE")"
tctl --auth-server="$PROXY_ADDR" --identity="$BOT_IDENTITY" create --force -f <<EOF
kind: db
version: v3
metadata:
  name: ${DYNAMIC_DB_NAME}
  description: Antithesis Postgres Database Access dynamic test target
  labels:
    test_template: db
    type: dynamic
spec:
  protocol: postgres
  uri: ${PG_HOST}:5432
  tls:
    mode: verify-full
    ca_cert: |
${db_ca_cert}
EOF

echo "db-client CA sync complete"
