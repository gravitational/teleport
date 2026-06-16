# Procedure: Write the Terraform

Produces the Terraform that, when applied, creates the AWS IAM role and policy,
optionally the AWS OIDC provider, the Teleport AWS OIDC integration, the
discovery_config, and for EC2 matchers the provision token. Applying it is a
separate procedure, `references/procedure-apply.md`.

## Procedure requirements

Decide where to write the Terraform. Take it from the user's request, otherwise
use the default.

| Input | From the request | Default |
|-------|------------------|---------|
| write location | A directory path, or an existing Terraform project to extend. | A new directory `teleport-discovery-aws/`. |

For a new directory, write `versions.tf` and `main.tf` as shown in the steps.
When extending an existing project, add the `module "aws_discovery"` block to it,
add any `required_providers` entries and `provider` blocks the project lacks, and
leave files it already has in place.

## Terraform module requirements

For each field, take the value from the first source that yields one, in this
order:

1. **Prompt** is the value stated in the user's request.
2. **Context** is a value already established earlier in this conversation.
3. **Tool use** runs the command in the last column.

Ask the user only when no source yields a value, and batch all such questions
into one message. Do not invent defaults. Run `tctl`/`tsh` against the target
cluster. If `TELEPORT_HOME` is set the commands use the right profile, otherwise
the user must `tsh login` first. Run independent tool-use commands in parallel.

| Field | Prompt / context | Tool use |
|-------|------------------|----------|
| `services` | The request: `["ec2"]`, `["eks"]`, or both. Drives each matcher's `types`. | none, ask if unstated. |
| `regions` | The request. Both EC2 and EKS matchers accept `["*"]` for all regions. | Read `$AWS_REGION`, then `$AWS_DEFAULT_REGION`, then `aws configure get region`. Ask if still empty. |
| `tags` | The request, as a map of key to list of values. Match-all `{"*": ["*"]}` only when the user wants every resource. | none, ask if unstated. |
| `kube_app_discovery` | The request, EKS only: enroll HTTP apps running inside the cluster. Default off. | none. |
| `proxy_addr` (host:port) | A proxy address already established. Must contain no scheme. | `tsh status -f json`, read `.profile_url`, strip the `https://` scheme. Falls back to `$TELEPORT_PROXY`. Cloud tenants use port `443`. |
| `cluster_version` | (none) | `tctl status`, the `Version` field. Pins the teleport provider's major version. |
| `deployment_type` (cloud / self-hosted) | The request. | Confirm with the user. Heuristic to confirm, not assume: a proxy host ending in `.teleport.sh` or `.cloud.gravitational.io` is Cloud. |
| `discovery_group` | (none) | Cloud → the fixed string `cloud-discovery-group`. Self-hosted → must equal the group of a running discovery service: `tctl inventory list --services=discovery`. If none runs, stop. A self-hosted cluster needs a deployed discovery service first, which this skill does not set up. |
| `existing_oidc_provider` (yes / no) | (none) | `aws iam list-open-id-connect-providers`, then for each ARN `aws iam get-open-id-connect-provider --open-id-connect-provider-arn <arn>` and read `.Url`. A match is `proxy_addr` with the scheme removed and `:443` stripped. When yes, set `create_aws_iam_openid_connect_provider = false`. |

For the plan's Environment block, also fetch the AWS account ID with
`aws sts get-caller-identity --query Account --output text` and whether an AWS
OIDC integration already exists with `tctl get integrations` (sub_kind
`aws-oidc`). The module derives the account itself and creates its own
integration, so these are for display only.

## Steps

1. Prepare the target. For the default, create `teleport-discovery-aws/`. For an
   existing project, use its directory. Outcome: a directory to write into.

2. Pin the providers in `versions.tf`. For a new directory, write the block. For
   an existing project, add only the entries it lacks. Set the teleport provider
   to the cluster's major version from `cluster_version`. The module requires the
   teleport provider `>= 18.8.0`.

   ```hcl
   terraform {
     required_version = ">= 1.5.7"
     required_providers {
       aws      = { source = "hashicorp/aws",  version = ">= 5.0" }
       http     = { source = "hashicorp/http", version = ">= 3.0" }
       tls      = { source = "hashicorp/tls",  version = ">= 4.0" }
       teleport = {
         source  = "terraform.releases.teleport.dev/gravitational/teleport"
         version = "~> <cluster-major>.0"
       }
     }
   }

   provider "aws" {
     region = "<a region from regions, or any region for wildcard matchers>"
   }

   provider "teleport" {
     addr = "<proxy_addr>"
     # A local apply needs no other fields. Other run environments add auth in the apply procedure.
   }
   ```

3. Add the module call. For a new directory, write `main.tf`. For an existing
   project, append the block to a `.tf` file. Outcome: the module call with the
   user's matchers and outputs for verification.

   ```hcl
   module "aws_discovery" {
     source = "terraform.releases.teleport.dev/teleport/discovery/aws"

     teleport_proxy_public_addr    = "<proxy_addr>"
     teleport_discovery_group_name = "<discovery_group>"

     aws_matchers = [
       # one object per matcher, see the schema below
     ]

     # Set this only when existing_oidc_provider = yes:
     # create_aws_iam_openid_connect_provider = false
   }

   output "integration_name"      { value = module.aws_discovery.teleport_integration_name }
   output "discovery_config_name" { value = module.aws_discovery.teleport_discovery_config_name }
   output "oidc_provider_arn"     { value = module.aws_discovery.aws_oidc_provider_arn }
   output "discovery_role_arn"    { value = module.aws_discovery.teleport_discovery_service_iam_role_arn }
   ```

## aws_matchers schema

Each object:

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `types` | list(string) | required | `["ec2"]`, `["eks"]`, or `["ec2","eks"]`. |
| `regions` | list(string) | `["*"]` | `["*"]` matches all regions for both EC2 and EKS. |
| `tags` | map(list(string)) | `{"*": ["*"]}` | Tag filter. |
| `kube_app_discovery` | bool | off | EKS only. Enroll HTTP apps running inside the cluster. |

Example, EC2 in all regions and EKS in us-east-1, each with its own tag filter:

```hcl
aws_matchers = [
  { types = ["ec2"], regions = ["*"],         tags = { env  = ["prod"] } },
  { types = ["eks"], regions = ["us-east-1"], tags = { team = ["platform"] }, kube_app_discovery = true },
]
```

## Conditional behavior the module already handles

- The AWS OIDC provider is created unless `create_aws_iam_openid_connect_provider = false`. Set it false when `existing_oidc_provider = yes` to avoid a 409 conflict.
- The provision token is created only when a matcher includes `ec2`.
- The OIDC audience is `discover.teleport` and the provider URL omits the port. The module handles both.
- The Teleport resources default to the name prefix `discovery` with the suffix `aws-account-<account-id>`. The outputs print the final names.

## Expected end state

- `versions.tf` and `main.tf` are written, or the module block and any missing
  provider entries are added to the existing project, with the four outputs
  defined.
- Apply the configuration per `references/procedure-apply.md`.
