#!/usr/bin/env bash
#
# Signs a Windows binary, cab file, or installer with the KMS-backed code signing
# certificate that setup.sh configures.
#
# Usage: sign-binary <unsigned-file> <signed-file>
#
# Windows paths are accepted as well as Linux ones, so that a Windows runner can
# call this through WSL with the paths its own build produced.

set -euo pipefail

# These two must match the constants at the top of setup.sh.
pkcs11_module_path=/usr/lib/x86_64-linux-gnu/pkcs11/aws_kms_pkcs11.so
pkcs11_token_label=signing-key

pkcs11_engine_path=/usr/lib/x86_64-linux-gnu/engines-3/pkcs11.so

# Written by setup.sh, which splits it out of the SSM certificate chain; must
# match intermediate_path there. PKCS#11 hands osslsigncode the leaf only, so
# without this the signature cannot be chained to a trusted root.
intermediate_path=/etc/aws-kms-pkcs11/signing-intermediate.crt

if [ "$#" -ne 2 ]; then
    echo "usage: $(basename "$0") <unsigned-file> <signed-file>" >&2
    exit 1
fi

# Only a Windows path needs translating, and only WSL can translate one.
to_linux_path() {
    if [ "${1#*:}" != "$1" ] && command -v wslpath > /dev/null; then
        wslpath "$1"
    else
        echo "$1"
    fi
}

unsigned_file="$(to_linux_path "$1")"
signed_file="$(to_linux_path "$2")"

echo "Signing $unsigned_file => $signed_file as AWS user $(aws sts get-caller-identity --query Arn --output text)"

if [ ! -r "$intermediate_path" ]; then
    echo "error: $intermediate_path is missing or unreadable; was setup.sh run?" >&2
    exit 1
fi

mkdir -p "$(dirname "$signed_file")"
export AWS_KMS_PKCS11_DEBUG=1

osslsigncode sign \
    -h sha256 \
    -pkcs11engine "$pkcs11_engine_path" \
    -pkcs11module "$pkcs11_module_path" \
    -pkcs11cert "pkcs11:token=$pkcs11_token_label" \
    -ac "$intermediate_path" \
    -ts http://timestamp.digicert.com \
    -in "$unsigned_file" \
    -out "$signed_file" \
    -verbose
