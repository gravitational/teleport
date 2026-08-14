#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-${OUT:-/etc/teleport-tls}}"
PUBLIC_ADDR="${2:-${PUBLIC_ADDR:-antithesis.teleport.local}}"
SANS="${SANS:-localhost 127.0.0.1 ${PUBLIC_ADDR} teleport}"
CERT_FILE="${CERT_FILE:-$OUT/teleport.pem}"
KEY_FILE="${KEY_FILE:-$OUT/teleport-key.pem}"

truthy() {
    case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
    esac
}

create_pg_client_cert() {
    local cn="$1"
    local csr="${PG_CERT_DIR}/client.csr"

    # mkcert doesn't have a way to set CN for client certs, so we generate a CSR and sign it with mkcert.
    openssl req -new -newkey rsa:2048 -nodes \
        -keyout "${PG_CERT_DIR}/client.key" \
        -out "$csr" \
        -subj "/CN=${cn}" \
        -addext "extendedKeyUsage=clientAuth" 1>&2

    mkcert -cert-file "${PG_CERT_DIR}/client.crt" -csr "$csr" 1>&2
    rm -f "$csr"
}

generate_pg_certs() {
    PG_CERT_DIR="${PG_CERT_DIR:-$OUT}"
    PG_CLIENT_CN="${PG_CLIENT_CN:-teleport}"
    PG_SERVER_NAMES="${PG_SERVER_NAMES:-pg localhost 127.0.0.1}"
    PG_SERVER_CERT_UID="${PG_SERVER_CERT_UID:-999}"
    PG_SERVER_CERT_GID="${PG_SERVER_CERT_GID:-999}"

    mkdir -p "$PG_CERT_DIR"
    cp "$CAROOT/rootCA.pem" "${PG_CERT_DIR}/ca.crt"

    # shellcheck disable=SC2086
    mkcert -cert-file "${PG_CERT_DIR}/server.crt" -key-file "${PG_CERT_DIR}/server.key" ${PG_SERVER_NAMES//,/ } 1>&2
    create_pg_client_cert "$PG_CLIENT_CN"

    chmod 0644 "${PG_CERT_DIR}/ca.crt" "${PG_CERT_DIR}/client.crt" "${PG_CERT_DIR}/server.crt"
    chmod 0600 "${PG_CERT_DIR}/client.key" "${PG_CERT_DIR}/server.key"
    chown "${PG_SERVER_CERT_UID}:${PG_SERVER_CERT_GID}" "${PG_CERT_DIR}/server.crt" "${PG_CERT_DIR}/server.key" || true
}

export CAROOT="$OUT/ca"
mkdir -p "$CAROOT" "$OUT"

mkcert -install 1>&2 || true

# shellcheck disable=SC2086
mkcert -cert-file "$CERT_FILE" -key-file "$KEY_FILE" ${SANS//,/ } 1>&2

cp "$CAROOT/rootCA.pem" "$OUT/rootCA.pem"

if truthy "${GENERATE_PG_CERTS:-${PG_CERTS:-0}}"; then
    generate_pg_certs
fi

echo 1>&2 "PKI generated in $OUT (public_addr=$PUBLIC_ADDR)"
echo 1>&2 "  cert: $CERT_FILE"
echo 1>&2 "  key:  $KEY_FILE"
echo 1>&2 "  SANs: $SANS"
ls -l "$OUT"
