#!/bin/bash

# Create the certs directory.
mkdir -p ./certs
cd certs/

# Create CA key and self-signed certificate.
openssl genpkey -algorithm RSA -out ca.key
openssl req -x509 -days 300 -new -key ca.key -out ca.crt -subj "/CN=root"

# Function to create certificates.
create_certificate() {
    local name="$1"
    local dns_name="$2"

    openssl genpkey \
        -algorithm RSA \
        -out "${name}.key"
    openssl req -new \
        -key "${name}.key" \
        -out "${name}.csr" \
        -subj "/CN=${dns_name}"
    openssl x509 -req \
        -in "${name}.csr" \
        -CA ca.crt \
        -CAkey ca.key \
        -out "${name}.crt" \
        -extfile <(printf "subjectAltName=DNS:${dns_name}") \
        -CAcreateserial \

    chmod 0600 "${name}.key"
}

# Create client certificate with SAN.
create_certificate "client" "teleport"

# Create server certificate with SAN.
create_certificate "node" "crdb,DNS:node,DNS:localhost,IP:127.0.0.1"

create_certificate "root" "root"

echo "Certificates and keys generated successfully."