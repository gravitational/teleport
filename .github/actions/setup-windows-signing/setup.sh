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

osslsigncode_url=https://github.com/mtrojnar/osslsigncode/releases/download/2.6/osslsigncode-2.6-ubuntu-22.04.zip
osslsigncode_sha256=3cc2474891605f29adbc58e08e0330052cda1f53540d2ce751783be5bd9088e1
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
packages=(unzip libssl-dev libengine-pkcs11-openssl libssl1.1 libjson-c4 openssl)
# Prefer an existing AWS CLI when present; otherwise install the distro package.
if ! command -v aws > /dev/null; then
    packages+=(awscli)
fi
$SUDO apt-get update
$SUDO apt-get install --no-install-recommends -y "${packages[@]}"
echo "::endgroup::"

echo "::group::Installing osslsigncode..."
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
curl -fsSL -o "$workdir/osslsigncode.zip" "$osslsigncode_url"
verify_checksum "$workdir/osslsigncode.zip" "$osslsigncode_sha256" "osslsigncode archive"
$SUDO unzip -o "$workdir/osslsigncode.zip" -d /usr
$SUDO chmod +x /usr/bin/osslsigncode
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
echo "Pulling the signing certificate from SSM at $WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH"
aws ssm get-parameter \
    --name "$WINDOWS_SIGNING_CERT_SSM_PARAMETER_PATH" \
    --query Parameter.Value --output text |
    $SUDO tee "$cert_path" > /dev/null

echo "Signing certificate:"
openssl x509 -in "$cert_path" -noout -subject -issuer -dates
openssl x509 -in "$cert_path" -noout -ext certificatePolicies

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
