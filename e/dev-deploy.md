# Deploying Local Builds to Teleport Cloud

Make targets in this repo are available to build teleport binaries locally and deploy them to an existing tenant on the Teleport Cloud staging cluster.

The initial implementation is derived from [cloud/RFD-0026](https://github.com/gravitational/cloud/blob/master/rfd/0026-Teleport-Release-Validation.md).

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
- jq
- kubectl
- tsh
- tc (from [gravitational/cloud/tc](https://github.com/gravitational/cloud/tree/master/tc)]

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
sso_role_name = tc-stage-ecr-role
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

Checks for valid logins and required permissions on platform.teleport.sh teleport cluster, staging kubernetes cluster and AWS ECR (Elastic Container Registry). Interactive steps are invoked only when an existing session is not found. No flags are required with this target.

```
make deploy-cloud-login
```

If the email configured in git doesn't contain a valid platform.teleport.sh SSO username, provide a value via environment variable:

```
TELEPORT_USER=first.last@goteleport.com make deploy-cloud-login
```

A tool from the [cloud repo](https://github.com/gravitational/cloud) is needed to patch your tenant on the staging cluster. If the `tc` binary is not available in your path, the source of the `tc` tool must be available in the path referenced by environment variable `TC_PATH`. The default value is `../../cloud/tc/cmd/tc`. Export a valid `TC_PATH` if the cloud repo is not cloned as a sibling of the `teleport` repo.

```
export TC_PATH=/src/cloud/tc/cmd/tc
make deploy-cloud-login
make TENANT=yourtenant deploy-cloud
```

### `deploy-cloud`

Minimally, provide a tenant name in the flag `TENANT`. The tenant name is the subdomain of your teleport cluster (e.g. for `mytenant.cloud.gravitational.io` provide `TENANT=mytenant`)

```
make TENANT=yourtenant deploy-cloud
```

This make target will build the `teleport` binary and copy it into an base release image. The tag for the base image is derived from `version.go`. If you've branched from `master` and no base image is available yet for a new major version, override by providing `BASE_IMAGE_TAG`. The default docker repo is `public.ecr.aws/teleport-ent` (override with `BASE_IMAGE_REPO`).

```
make TENANT=yourtenant BASE_IMAGE_TAG=10.1.4 deploy-cloud
```

Deploys the provided release to the tenant. The default docker repo is `public.ecr.aws/teleport-ent` (override with `BASE_IMAGE_REPO`).

```
make TENANT=yourtenant RELEASE=14.0.0 deploy-cloud
```

Uses the staging repo for non-prod releases by overriding `BASE_IMAGE_REPO`.

```
make BASE_IMAGE_REPO=public.ecr.aws/gravitational-staging/teleport-ent-distroless RELEASE=18.0.0-alpha.1 TENANT=yourtenant deploy-cloud
```

If your tenant's auth region is not `us-west-2`, you'll also need to override `KUBE_AUTH_CLUSTER`. For example, if the auth region was `eu-central-1`:

```
make KUBE_AUTH_CLUSTER=tc-staging-cs-01-euc1 TENANT=yourtenant deploy-cloud
```

Linux workstations having a recent `glibc` version will produce binaries incompatible with our release images (script will warn & fail). Generate binaries using dockerized build then include the flag `CLOUD_SKIP_BUILD` to use the existing binary.

```
make -C ../build.assets release-centos7
make TENANT=yourtenant CLOUD_SKIP_BUILD=1 deploy-cloud
```

Flag variables are available to skip deployment and monitoring stages:
- `CLOUD_SKIP_DEPLOY=1`: new docker image will be pushed, but the tenant will not be modified to use the new image.
- `CLOUD_SKIP_ROLLOUT=1`: skip monitoring of new pod rollout, deploy script terminates after tenant is modified.

If the build fails with `cgo` errors, provide a C compiler that targets `x86_64/linux`:
```
# compiler in PATH
CC=x86_64-unknown-linux-gnu-gcc make TENANT=yourtenant deploy-cloud

# darwin/arm64 w/homebrew messense/macos-cross-toolchains
CC=/opt/homebrew/bin/x86_64-unknown-linux-gnu-gcc make TENANT=yourtenant deploy-cloud

# darwin/x86 w/homebrew messense/macos-cross-toolchains
CC=/usr/local/bin/x86_64-unknown-linux-gnu-gcc make TENANT=yourtenant deploy-cloud
```
