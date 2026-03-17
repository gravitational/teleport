# Deploying Local Builds to Teleport Cloud

Make targets in this repo are available to build teleport binaries locally and deploy them to new or existing tenants on the Teleport Cloud staging cluster.
Start with the prerequisites below, then use the New User Quickstart section for the fastest path.

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

#### Errors
Dependencies between versions might cause issues when building your binaries. E.g. rustc version required for package installation.
```shell
error: cannot install package `wasm-bindgen-cli 0.2.108`, it requires rustc 1.82 or newer, while the currently active rustc version is 1.81.0
```
cd into main teleport repo, confirm your version
```shell
make --no-print-directory -C build.assets print-rust-version
rustup override unset            # if you have a stale directory override
rustup toolchain install <version-from-command>
make rustup-set-version
```

#### tc

The `tc` tool is used to interact with Teleport Cloud environments. It provides commands for managing tenants, deployments, and other resources within the Teleport Cloud. The source code is available in the [gravitational/cloud]((https://github.com/gravitational/cloud/tree/master/tc) repo.

If the `tc` binary is not available in your path, the source of the `tc` tool must be available in the path referenced by environment variable `TC_PATH`. The default value is `../../cloud/tc/cmd/tc`. Export a valid `TC_PATH` if the cloud repo is not cloned as a sibling of the `teleport` repo.

```shell
export TC_PATH=/src/cloud/tc/cmd/tc
```

Alternatively, to use a `tc` binary built from source, run the following commands from the `cloud` repo:

```shell
# Navigate to the cloud repo
cd ../../cloud
# Build the tc binary
go build -o dist/tc ./tc/cmd/tc
# Link the binary to a location in your PATH
ln -s dist/tc ~/.local/bin/tc
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

### New User Quickstart

First, run `make deploy-cloud-login`. It verifies and establishes the authenticated sessions required for Cloud deployments.

#### Minimal first run

```shell
# 1) Verify/login all required systems (sessions are refreshed if needed)
make deploy-cloud-login

# 2) Build Linux/amd64 teleport binary
#    CC is required on macOS ARM; adjust the path for your toolchain installation.
make CC=/opt/homebrew/bin/x86_64-unknown-linux-gnu-gcc build-cloud-teleport-binary

# 3a) Create a new tenant
make TENANT=yourtenant create-cloud

# 3b) Or deploy to existing tenant
make TENANT=yourtenant \
      BASE_IMAGE_REPO=public.ecr.aws/gravitational/teleport-ent-distroless \
      BASE_IMAGE_TAG=18.7.3 \
      CC=/opt/homebrew/bin/x86_64-unknown-linux-gnu-gcc \
      deploy-cloud
```

`BASE_IMAGE_REPO` and `BASE_IMAGE_TAG` are optional and default to the published container image matching the local version. On `master`, the scripts attempt to auto-resolve a recent nightly staging image.

#### Required local binary path

When not using `RELEASE=...`, the script expect:

- `build/teleport` (or `${BUILDDIR}/teleport` if `BUILDDIR` is set)

`create-cloud` does not build this binary automatically. If missing, build it first:

```shell
make CC=/opt/homebrew/bin/x86_64-unknown-linux-gnu-gcc build-cloud-teleport-binary
```

#### Which target should I use?

- `make create-cloud`
  - Creates a new tenant.
  - Expects a prebuilt binary at `build/teleport`.
- `make deploy-cloud`
  - Patches an existing tenant.
  - Builds `build/teleport` by default unless `CLOUD_SKIP_BUILD=1` is set.
- `make build-cloud-teleport-binary`
  - Builds the required linux/amd64 `teleport` binary for image patching.
- `make deploy-cloud-login`
  - Verifies access to `platform.teleport.sh`, the staging tenant-management, auth Kubernetes contexts, and the staging AWS account/ECR repositories used by the cloud deploy.

### Common Flags

These make targets are controlled primarily through environment variables passed inline:

```shell
make TENANT=yourtenant BASE_IMAGE_TAG=18.7.1 deploy-cloud
```

The most commonly used flags are:

| Flag | Used by | What it does                                                                                   |
| --- | --- |------------------------------------------------------------------------------------------------|
| `TENANT` | `create-cloud`, `deploy-cloud`, `delete-cloud` | Tenant subdomain name (for `mytenant.cloud.gravitational.io`, use `TENANT=mytenant`).          |
| `RELEASE` | `create-cloud`, `deploy-cloud` | Uses an existing published Teleport release instead of a local `build/teleport` binary.        |
| `BASE_IMAGE_REPO` | `create-cloud`, `deploy-cloud` | Base container image repository to pull before replacing `/usr/local/bin/teleport`.            |
| `BASE_IMAGE_TAG` | `create-cloud`, `deploy-cloud` | Base image tag. If not set, defaults to version derived from `version.go`.                     |
| `TARGET_IMAGE_REPO` | `create-cloud`, `deploy-cloud` | Destination image repository for the patched image. Defaults to the staging ECR repo used for local cloud builds.               |
| `CLOUD_SKIP_BUILD` | `deploy-cloud` | Skips local binary build and requires an existing `build/teleport`.                            |
| `CLOUD_SKIP_DEPLOY` | `deploy-cloud` | Builds and pushes image, but does not patch tenant to use it.                                  |
| `CLOUD_SKIP_ROLLOUT` | `deploy-cloud` | Patches tenant but skips rollout monitoring.                                                   |
| `KUBE_AUTH_CLUSTER` | `deploy-cloud`, `deploy-cloud-login` | Cluster name used for rollout monitoring checks.            |
| `REGION` | `create-cloud` | Region where new tenant auth pods are created.                                                 |
| `CC` | `build-cloud-teleport-binary`, `deploy-cloud` | Linux cross-compiler for macOS CGO cross-builds (for example `x86_64-unknown-linux-gnu-gcc`).  |
| `TC_PATH` | all cloud targets | Path to `cloud/tc/cmd/tc` source when `tc` binary is not in `PATH`.                            |
| `TELEPORT_HOME` | all cloud targets | Path to Teleport profile directory used by `tsh`/`tc` auth (`$HOME/.tsh_platform` by default). |

### How Base Image Selection Works

When not using `RELEASE=...`, the scripts patch a container image by replacing `/usr/local/bin/teleport` with your local binary.

- On release branches, `BASE_IMAGE_REPO` defaults to `public.ecr.aws/gravitational/teleport-ent-distroless` and `BASE_IMAGE_TAG` defaults to the local Teleport version.
- On `master`, if `BASE_IMAGE_TAG` is not set, the scripts attempt to auto-resolve a recent nightly image from the staging container repo.
- If you want a specific image, set both `BASE_IMAGE_REPO` and `BASE_IMAGE_TAG` explicitly.

Examples:

```shell
# Use your locally generated teleport binary to patch the targeted BASE_IMAGE_TAG from BASE_IMAGE_REPO.
# In this specific example, a public image generated from a nightly build run
make TENANT=yourtenant \
  BASE_IMAGE_REPO=public.ecr.aws/gravitational-staging/teleport-ent-distroless \
  BASE_IMAGE_TAG=19.0.0-dev-nightly20260304-1-eef4a4c \
  deploy-cloud

# Tenant auth region is not us-west-2: override rollout monitoring cluster.
make TENANT=yourtenant KUBE_AUTH_CLUSTER=tc-staging-cs-01-euc1 deploy-cloud

# Build/push image only; do not patch tenant.
make TENANT=yourtenant CLOUD_SKIP_DEPLOY=1 deploy-cloud

# Deploy an existing release image directly. 
make TENANT=yourtenant RELEASE=18.7.1 deploy-cloud

# Create a tenant using default base-image selection for the current branch.
make TENANT=yourtenant create-cloud
```

### Teleport Version caveats

To bump the Teleport version of an existing tenant using a local build, edit the `VERSION` variable in the `Makefile`, and running `make version` prior to making a build.
Note: a tenant must have already been deployed, see the `create-cloud` section.

However, `VERSION` is not used with `create-cloud` or `deploy-cloud` when deploying any of the published releases.
For that, `RELEASE` (e.g., `RELEASE=18.7.0`) is used and the version is determined by the `RELEASE` value provided.

#### Version compatibility
Teleport does not allow major version downgrade.
If you deploy `vN`, you will not be able to deploy `vN-1`.

This also applies to `master`: after deploying from `master` (eg `v19.0.0-dev`) you will not be able to go back to a production release in that tenant.

#### CDN assets (install and set up scripts)

Some features, like integration set up scripts and server discovery, rely on CDN assets.
The URL for those assets will be based on the cluster version.

If you use `master`, you will not be able to use those scripts (ie, the CDN server does not have non-release artifacts).

In this case, you have to change the teleport version to a production release (ideally, the latest) so that you can access those artifacts.

### `deploy-cloud-login`

Checks for valid logins and required permissions on `platform.teleport.sh`, the staging Kubernetes clusters, and AWS ECR. Interactive steps are invoked only when an existing session is not found. No flags are required with this target.

```
make deploy-cloud-login
```

`deploy-cloud-login` also validates that rollout monitoring can reach `deployment/teleport-auth` in the auth cluster context used by `deploy-cloud`. If this check fails, the script prints context regeneration commands (`tsh login` and `tsh kube login --all`) to run before retrying.

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

### `create-cloud`

Minimally, provide a tenant name in the flag `TENANT`. The tenant name is the subdomain of your teleport cluster (e.g. for `mytenant.cloud.gravitational.io` provide `TENANT=mytenant`)

```
make TENANT=yourtenant create-cloud
```

Specify a region to have the auth pods created outside of the default (us-west-2).

```
make TENANT=yourtenant REGION=us-east-1 create-cloud
```

Creates new tenant with the provided release. The default docker repo is `public.ecr.aws/teleport-ent` (override with `BASE_IMAGE_REPO`).

```
make TENANT=yourtenant RELEASE=14.0.0 create-cloud
```

`create-cloud` expects a prebuilt Linux/amd64 `teleport` binary at `build/teleport`. Base image selection follows the shared rules in [How Base Image Selection Works](#how-base-image-selection-works).

```
make TENANT=yourtenant BASE_IMAGE_TAG=10.1.4 create-cloud
```

Uses the staging repo for non-prod releases by overriding `BASE_IMAGE_REPO`.

```
make BASE_IMAGE_REPO=public.ecr.aws/gravitational-staging/teleport-ent-distroless RELEASE=18.0.0-alpha.1 TENANT=yourtenant create-cloud
```

### `delete-cloud`


Minimally, provide a tenant name in the flag `TENANT`. The tenant name is the subdomain of your teleport cluster (e.g. for `mytenant.cloud.gravitational.io` provide `TENANT=mytenant`)

```
make TENANT=yourtenant delete-cloud
```

### `deploy-cloud`

Minimally, provide a tenant name in the flag `TENANT`. The tenant name is the subdomain of your teleport cluster (e.g. for `mytenant.cloud.gravitational.io` provide `TENANT=mytenant`)

```
make TENANT=yourtenant deploy-cloud
```

By default, `deploy-cloud` builds the local `teleport` binary and patches it into a base image. Base image selection follows the shared rules in [How Base Image Selection Works](#how-base-image-selection-works).

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
