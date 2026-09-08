#!/usr/bin/env bash
#
# Installs the Windows code signing toolchain (osslsigncode and aws-kms-pkcs11),
# points it at the KMS-backed code signing certificate, and installs the sign-binary
# helper alongside it on PATH.
#
# Runs on Linux, either natively on a Linux runner or inside WSL on a Windows
# runner. WSL is installed as root while Linux runners are not, hence $SUDO.
#
# Required environment:
#   WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH  SSM parameter holding the certificate
#   WINDOWS_SIGNING_KEY_ARN                  KMS key or alias holding the signing key
# Optional:
#   WINDOWS_SIGNING_AWS_REGION               defaults to us-west-2

set -euo pipefail

: "${WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH:?must be set}"
: "${WINDOWS_SIGNING_KEY_ARN:?must be set}"
region="${WINDOWS_SIGNING_AWS_REGION:-us-west-2}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# These two must match the constants at the top of sign-binary.sh.
pkcs11_module_path=/usr/lib/x86_64-linux-gnu/pkcs11/aws_kms_pkcs11.so
pkcs11_token_label=signing-key

# This specific directory is required by aws-kms-pkcs11.
config_dir=/etc/aws-kms-pkcs11

# Need osslsigncode >= 2.7 because 2.6 corrupts signed CAB files. The last upstream
# prebuilt binary compatible with Ubuntu 22.04 was version 2.6, as a result we build
# a fixed version from source.
#
# Pinned by commit rather than tag or tarball: GitHub's generated archives are
# not guaranteed byte-stable, but a commit hash is content-addressed.
osslsigncode_repo=https://github.com/mtrojnar/osslsigncode.git
osslsigncode_commit=76ee550c9d3b9f0e559f044e18136b74c167fef2 # tag 2.9
aws_kms_pkcs11_url=https://github.com/JackOfMostTrades/aws-kms-pkcs11/releases/download/v0.0.10/aws_kms_pkcs11.x86_64.so
aws_kms_pkcs11_sha256=93c10632f11c6936373e9ff3a9877624320aa6c24bdcdff3da39cda519241ec7

if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
else
    SUDO=sudo
fi

verify_checksum() {
    local file="$1" want="$2" what="$3"
    if ! echo "$want  $file" | sha256sum --check --status; then
        echo "error: the downloaded $what did not match its expected checksum" >&2
        exit 1
    fi
}

echo "::group::Installing dependencies..."
# libssl1.1 and libjson-c4 are required by aws-kms-pkcs11 and are not available
# on 22.04, so pull them from the 20.04 security repo.
echo "deb http://security.ubuntu.com/ubuntu focal-security main" |
    $SUDO tee /etc/apt/sources.list.d/focal-security.list > /dev/null

# cabextract verifies that a signed cabinet is still readable; the rest from
# build-essential onwards are only needed to build osslsigncode from source.
packages=(
    unzip libssl-dev libengine-pkcs11-openssl libssl1.1 libjson-c4 openssl
    cabextract git
    build-essential cmake libcurl4-openssl-dev zlib1g-dev pkg-config
)
# Prefer an existing AWS CLI when present; otherwise install the distro package.
if ! command -v aws > /dev/null; then
    packages+=(awscli)
fi
$SUDO apt-get update
$SUDO apt-get install --no-install-recommends -y "${packages[@]}"
echo "::endgroup::"

echo "::group::Building osslsigncode..."
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
git -C "$workdir" init --quiet osslsigncode
git -C "$workdir/osslsigncode" fetch --quiet --depth 1 "$osslsigncode_repo" "$osslsigncode_commit"
git -C "$workdir/osslsigncode" checkout --quiet FETCH_HEAD
# Make sure the fetched revision matches the pinned osslsigncode commit
got="$(git -C "$workdir/osslsigncode" rev-parse HEAD)"
if [ "$got" != "$osslsigncode_commit" ]; then
    echo "error: expected osslsigncode commit $osslsigncode_commit but got $got" >&2
    exit 1
fi
cmake -S "$workdir/osslsigncode" -B "$workdir/build" -DCMAKE_BUILD_TYPE=Release > /dev/null
cmake --build "$workdir/build" -j "$(nproc)" > /dev/null
$SUDO install -m 0755 "$workdir/build/osslsigncode" /usr/bin/osslsigncode
osslsigncode --version
echo "::endgroup::"

echo "::group::Installing aws-kms-pkcs11..."
curl -fsSL -o "$workdir/aws_kms_pkcs11.so" "$aws_kms_pkcs11_url"
verify_checksum "$workdir/aws_kms_pkcs11.so" "$aws_kms_pkcs11_sha256" "aws-kms-pkcs11 module"
$SUDO mkdir -p "$(dirname "$pkcs11_module_path")"
$SUDO install -m 0644 "$workdir/aws_kms_pkcs11.so" "$pkcs11_module_path"
echo "::endgroup::"

echo "::group::Configuring the signing key..."
$SUDO mkdir -p "$config_dir"
cert_path="$config_dir/signing-cert.crt"
intermediate_path="$config_dir/signing-intermediate.crt"
echo "Pulling the signing certificate chain from SSM at $WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH"
aws ssm get-parameter \
    --name "$WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH" \
    --query Parameter.Value --output text > "$workdir/chain.pem"

# The parameter holds the full chain: leaf, issuing intermediate, root. Split the
# first two out. aws-kms-pkcs11 presents only the first certificate through
# PKCS#11, so the intermediate has to reach osslsigncode separately via -ac --
# without it the signature carries the leaf alone and cannot be chained to a
# trusted root. The root is deliberately not extracted: it belongs in the trust
# store, not in the signature.
awk '/BEGIN CERTIFICATE/{n++} n==1' "$workdir/chain.pem" | $SUDO tee "$cert_path" > /dev/null
awk '/BEGIN CERTIFICATE/{n++} n==2' "$workdir/chain.pem" | $SUDO tee "$intermediate_path" > /dev/null

if ! openssl x509 -in "$intermediate_path" -noout 2> /dev/null; then
    echo "error: no intermediate certificate found in $WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH;" \
        "it must hold the leaf followed by its issuing intermediate" >&2
    exit 1
fi
if [ "$(openssl x509 -in "$cert_path" -noout -issuer_hash)" \
    != "$(openssl x509 -in "$intermediate_path" -noout -subject_hash)" ]; then
    echo "error: the second certificate in $WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH is not" \
        "the issuer of the first; the chain is out of order or the leaf was reissued" >&2
    exit 1
fi

echo "Signing certificate:"
openssl x509 -in "$cert_path" -noout -subject -issuer -dates
openssl x509 -in "$cert_path" -noout -ext certificatePolicies
echo "Issuing intermediate:"
openssl x509 -in "$intermediate_path" -noout -subject -dates

kms_key_id="$(aws kms describe-key \
    --key-id "$WINDOWS_SIGNING_KEY_ARN" \
    --query KeyMetadata.KeyId --output text)"
echo "Using KMS key ID $kms_key_id for key $WINDOWS_SIGNING_KEY_ARN"

$SUDO tee "$config_dir/config.json" > /dev/null <<CONFIG
{
  "slots": [
    {
      "label": "$pkcs11_token_label",
      "kms_key_id": "$kms_key_id",
      "aws_region": "$region",
      "certificate_path": "$cert_path"
    }
  ]
}
CONFIG
echo "::endgroup::"

echo "::group::Installing sign-binary..."
$SUDO install -m 0755 "$script_dir/sign-binary.sh" /usr/local/bin/sign-binary
echo "Installed sign-binary to /usr/local/bin/sign-binary"
echo "::endgroup::"
