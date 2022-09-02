# Deploying Local Builds to Teleport Cloud

Make targets in this repo are available to build teleport binaries locally and deploy them to an existing tenant on the Teleport Cloud staging cluster.

The inital implementation is derived from [cloud/RFD-0026](https://github.com/gravitational/cloud/blob/master/rfd/0026-Teleport-Release-Validation.md).

## Pre-requisites

### Access

The following systems need to be accessible:
- Teleport Cloud staging cluster - bring-your-own tenant subdomain on cloud.gravitational.io
- Cloud team staging AWS account - push docker images to Elastic Container Registry (AWS ECR)
- platform.teleport.sh - provides role-base access to the following systems:
  - Sales Center staging - create a tenant on staging cluster
  - Teleport Cloud staging kubernetes cluster - modify `tenant` resources

### Tools 

The following CLI tools need to be available:
- aws (v2+ required)
- docker
- kubectl
- tsh

On macOS a compiler toolchain targeting `x86_64/linux` is required to support `cgo` directives.

  ```shell
  brew tap messense/macos-cross-toolchains
  brew install x86_64-unknown-linux-gnu
  ```

### Configuration

<details>
<summary>Core Team</summary>

Ensure the following profiles are defined in `~/.aws/config`:

```ini
[profile tc-stage-core]
sso_start_url = https://d-92670253d5.awsapps.com/start
sso_region = us-west-2
sso_account_id = 599519581022
sso_role_name = AWS-TeleportCloud-Stage-ECR
region = us-west-2
output = json

[profile tc-stage-ecr]
role_arn = arn:aws:iam::599519581022:role/coreteam-localbuilds-ecr-role
source_profile = tc-stage-core
region = us-west-2
output = json
max_attempts = 1
```
</details>

<details>
<summary>Cloud Team</summary>

The AWS profiles shown above for Core team can be used by the Cloud team.

To use the Cloud team AWS profiles instead, set environment variables to select the staging admin profile and to override the SSO login profile before invoking the `deploy-cloud-login` target:

```shell
export AWS_PROFILE=tc-stage-admin
export AWS_SSO_PROFILE=tc-stage-ro
```
</details>

## Usage

### `deploy-cloud-login`

Checks for valid logins and required permissions on platform.teleport.sh teleport cluster, staging kubernetes cluter and AWS ECR (Elastic Container Registry). Interactive steps are invoked only when an existing session is not found. No flags are required with this target.

```
make deploy-cloud-login
```

If the email configured in git doesn't contain a valid platform.teleport.sh SSO username, provide a value via environment variable:

```
TELEPORT_USER=first.last@goteleport.com make deploy-cloud-login
```

### `deploy-cloud`

Minimally, provide a tenant name in the flag `TENANT`.

```
make TENANT=yourtenant deploy-cloud
```

The docker repository is located is us-west-2. If uploads are slow, use a base image instead of a Dockerfile - only the `teleport` binary will be transferred after the first push. This cuts down upload time by ~2/3rds.

```
make TENANT=yourtenant CLOUD_DOCKERFILE=- BASE_IMAGE_TAG=10.1.4 deploy-cloud
```

Base images come from the `teleport-ent` repo on quay.io by default. To override, provide a value for "BASE_IMAGE_REPO" along with the image tag.

```
make TENANT=yourtenant CLOUD_DOCKERFILE=- BASE_IMAGE_REPO=public.ecr.aws/gravitational/teleport-ent BASE_IMAGE_TAG=10.1.4 deploy-cloud
```

If the build fails with `cgo` errors, provide a C compiler that targets `x86_64/linux`.

```
CC=/opt/homebrew/bin/x86_64-unknown-linux-gnu-gcc make TENANT=yourtenant deploy-cloud
```