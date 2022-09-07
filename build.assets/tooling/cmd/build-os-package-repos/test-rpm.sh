#!/bin/bash
# shellcheck disable=SC2016,SC1004,SC2174,SC2155

set -xeu

# These must be set for the script to run
: "$AWS_ACCESS_KEY_ID"
: "$AWS_SECRET_ACCESS_KEY"
: "$AWS_SESSION_TOKEN"

ART_VERSION_TAG="8.3.15"
ARTIFACT_PATH="/go/artifacts"
CACHE_DIR="/mnt/createrepo_cache"
GNUPGHOME="/tmpfs/gnupg"
REPO_S3_BUCKET="fred-test1"
BUCKET_CACHE_PATH="/mnt/bucket"
export AWS_REGION="us-west-2"

: '
Run command:
docker run \
    --rm -it \
    -v "$(git rev-parse --show-toplevel)":/go/src/github.com/gravitational/teleport/ \
    -v "$HOME/.aws":"/root/.aws" \
    -e AWS_PROFILE="$AWS_PROFILE" \
    -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
    -e AWS_SESSION_TOKEN="$AWS_SESSION_TOKEN" \
    -e DEBIAN_FRONTEND="noninteractive" \
    golang:1.18.4-bullseye /go/src/github.com/gravitational/teleport/build.assets/tooling/cmd/build-os-package-repos/test-rpm.sh
'

# Download the artifacts
apt update
apt install -y wget
mkdir -pv "$ARTIFACT_PATH"
cd "$ARTIFACT_PATH"
wget "https://get.gravitational.com/teleport-${ART_VERSION_TAG}-1.x86_64.rpm"
wget "https://get.gravitational.com/teleport-${ART_VERSION_TAG}-1.arm64.rpm"
wget "https://get.gravitational.com/teleport-${ART_VERSION_TAG}-1.i386.rpm"
wget "https://get.gravitational.com/teleport-${ART_VERSION_TAG}-1.arm.rpm"

apt install -y createrepo-c gnupg
mkdir -pv "$CACHE_DIR"
mkdir -pv -m0700 "$GNUPGHOME"
chown -R root:root "$GNUPGHOME"
export GPG_TTY=$(tty)
gpg --batch --gen-key <<EOF
Key-Type: 1
Key-Length: 2048
Subkey-Type: 1
Subkey-Length: 2048
Name-Real: Test RPM key
Name-Email: test@rpm.key
Expire-Date: 0
%no-protection
EOF
cd "/go/src/github.com/gravitational/teleport/build.assets/tooling"
export VERSION="v${ART_VERSION_TAG}"
export RELEASE_CHANNEL="stable"
go run ./cmd/build-os-package-repos yum -bucket "$REPO_S3_BUCKET" -local-bucket-path \
    "$BUCKET_CACHE_PATH" -artifact-version "$VERSION" -release-channel "$RELEASE_CHANNEL" \
    -artifact-path "$ARTIFACT_PATH" -log-level 4 -cache-dir "$CACHE_DIR"