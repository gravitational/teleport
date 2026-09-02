#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-${OUT:-/etc/teleport-tls}}"
PUBLIC_ADDR="${2:-${PUBLIC_ADDR:-antithesis.teleport.local}}"
SANS="${SANS:-localhost 127.0.0.1 ${PUBLIC_ADDR} teleport}"
CERT_FILE="${CERT_FILE:-$OUT/teleport.pem}"
KEY_FILE="${KEY_FILE:-$OUT/teleport-key.pem}"
LICENSE_FILE="${LICENSE_FILE:-/license/license.pem}"

create_pg_client_cert() {
    local cn="$1" cert_dir="$2"
    local csr="${cert_dir}/client.csr"

    openssl req -new -newkey rsa:2048 -nodes \
        -keyout "${cert_dir}/client.key" \
        -out "$csr" \
        -subj "/CN=${cn}" \
        -addext "extendedKeyUsage=clientAuth" 1>&2

    mkcert -cert-file "${cert_dir}/client.crt" -csr "$csr" 1>&2
    rm -f "$csr"
}

generate_pg_certs() {
    local cert_dir="$1" server_names="$2" client_cn="$3" uid="$4" gid="$5"

    mkdir -p "$cert_dir"
    cp "$CAROOT/rootCA.pem" "${cert_dir}/ca.crt"

    # shellcheck disable=SC2086
    mkcert -cert-file "${cert_dir}/server.crt" -key-file "${cert_dir}/server.key" ${server_names//,/ } 1>&2

    if [[ -n "$client_cn" ]]; then
        create_pg_client_cert "$client_cn" "$cert_dir"
        chmod 0644 "${cert_dir}/client.crt"
        chmod 0600 "${cert_dir}/client.key"
    fi

    chmod 0644 "${cert_dir}/ca.crt" "${cert_dir}/server.crt"
    chmod 0600 "${cert_dir}/server.key"
    chown "${uid}:${gid}" "${cert_dir}/server.crt" "${cert_dir}/server.key" 2>/dev/null || true

    echo 1>&2 "Postgres PKI generated in $cert_dir (server_names=[$server_names] client_cn=${client_cn:-<none>})"
    ls -l "$cert_dir" 1>&2
}

generate_backend_pg_certs() {
    local cert_dir="${BACKEND_PG_CERT_DIR:-${PG_CERT_DIR:-$OUT}}"
    local server_names="${BACKEND_PG_SERVER_NAMES:-${PG_SERVER_NAMES:-pg localhost 127.0.0.1}}"
    local client_cn="${BACKEND_PG_CLIENT_CN:-${PG_CLIENT_CN:-teleport}}"
    local uid="${BACKEND_PG_CERT_UID:-${PG_SERVER_CERT_UID:-999}}"
    local gid="${BACKEND_PG_CERT_GID:-${PG_SERVER_CERT_GID:-999}}"
    generate_pg_certs "$cert_dir" "$server_names" "$client_cn" "$uid" "$gid"
}

generate_proxy_cert() {
    export CAROOT="$OUT/ca"
    mkdir -p "$CAROOT" "$OUT"

    mkcert -install 1>&2 || true

    # shellcheck disable=SC2086
    mkcert -cert-file "$CERT_FILE" -key-file "$KEY_FILE" ${SANS//,/ } 1>&2
    cp "$CAROOT/rootCA.pem" "$OUT/rootCA.pem"

    echo 1>&2 "Proxy PKI generated in $OUT (public_addr=$PUBLIC_ADDR)"
    echo 1>&2 "  cert: $CERT_FILE"
    echo 1>&2 "  key:  $KEY_FILE"
    echo 1>&2 "  SANs: $SANS"
}

generate_license() {
    local license_dir
    license_dir="$(dirname "$LICENSE_FILE")"
    mkdir -p "$license_dir"

    helper_generate_license --out "$LICENSE_FILE"

    echo 1>&2 "Teleport license generated at $LICENSE_FILE"
    ls -l "$license_dir" 1>&2
}

# main
generate_proxy_cert
generate_license
generate_backend_pg_certs

ls -lR "$OUT" 1>&2
