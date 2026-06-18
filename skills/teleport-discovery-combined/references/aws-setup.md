# AWS Discovery Setup

## Gather requirements

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `proxy_addr` | `$TSH status --format=json`, `active.profile_url` with the `https://` scheme stripped, such as `example.teleport.sh:443` | Ask |
| `cluster_version` | `$TCTL status` `Version` field, such as `18.8.0` | Ask |
| `deployment` | `cloud` when `proxy_addr`'s host ends in `.teleport.sh` or `.cloud.gravitational.io`, else `self-hosted` | none |
| `services` | none | Ask: `ec2`, `eks`, or both |
| `regions` | `$AWS_REGION`, then `$AWS_DEFAULT_REGION`, then `aws configure get region` | Ask; `["*"]` matches all regions |
| `tags` | none | Ask; use `{"*": ["*"]}` only when the user wants every resource |
| `kube_app_discovery` | none | Off; EKS only |
| `aws_account_id` | `aws sts get-caller-identity --query Account --output text` | Omit from the plan |
| `existing_oidc_provider` | `aws iam list-open-id-connect-providers`, then `aws iam get-open-id-connect-provider --open-id-connect-provider-arn <arn>` and read `.Url`; `yes` when `.Url`, with any scheme and port removed, equals `proxy_addr`'s host with its port removed | `no` |
| `existing_integration` | `$TCTL get integrations`; a match has `sub_kind: aws-oidc` | Omit from the plan |
| `discovery_group` | `cloud`: `cloud-discovery-group`. `self-hosted`: confirm a service runs with `$TCTL inventory list --services=discovery`, and stop if none runs | Ask for the `discovery_group` set in the Discovery Service's `teleport.yaml` |
| `write_location` | none | A new `teleport-discovery-aws/` directory |

AWS discovery requires the Teleport provider `>= 18.8.0`. If `cluster_version` is below
`18.8.0`, stop: "AWS discovery requires Teleport 18.8.0 or later. This cluster is
v`<cluster_version>`."

## Write location

Into a new project, write a fresh module: a new `teleport-discovery-aws/` directory with
`versions.tf` and `main.tf`. Into an existing Terraform project, integrate following its
structure. If the project already declares the `module "aws_discovery"` block, read it,
pre-populate the gathered fields from its current values, and edit that block in place.

## Show the plan

Present this with real values, then wait for approval unless the request set
`auto_approve: true`.

```
## Environment
Cluster:         <deployment>, <proxy_addr>, v<cluster_version>
AWS account:     <aws_account_id>
Discovery group: <discovery_group>
Existing aws-oidc integration: <name | none>
Existing AWS OIDC provider for this proxy: <yes | no>
Write location:  <path>, <new project | extend existing>

## Matchers
<one line per matcher: types, regions, tags, kube_app_discovery>

## Plan
Teleport resources: aws-oidc integration, discovery_config, and a provision token when an ec2 matcher is present
AWS resources:      IAM role, IAM policy, and the OIDC provider unless existing_oidc_provider is yes
Files:              <files written or edited>

Approve? (y/n)
```

## Write the Terraform

Declare the provider requirements and configuration. The teleport provider must be
`>= 18.8.0`, the discovery module's minimum:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    aws      = { source = "hashicorp/aws",  version = ">= 5.0" }
    http     = { source = "hashicorp/http", version = ">= 3.0" }
    tls      = { source = "hashicorp/tls",  version = ">= 4.0" }
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= 18.8.0"
    }
  }
}

provider "aws" {
  region = "<a region from regions; any region for wildcard matchers>"
}

provider "teleport" {
  addr = "<proxy_addr>"
}
```

Write the discovery module:

```hcl
module "aws_discovery" {
  source = "terraform.releases.teleport.dev/teleport/discovery/aws"

  teleport_proxy_public_addr    = "<proxy_addr>"
  teleport_discovery_group_name = "<discovery_group>"

  aws_matchers = [
    # one object per matcher, per the schema below
  ]

  # Set to false only when existing_oidc_provider is yes, to avoid a 409 conflict on apply:
  # create_aws_iam_openid_connect_provider = false
}

output "integration_name"      { value = module.aws_discovery.teleport_integration_name }
output "discovery_config_name" { value = module.aws_discovery.teleport_discovery_config_name }
output "oidc_provider_arn"     { value = module.aws_discovery.aws_oidc_provider_arn }
output "discovery_role_arn"    { value = module.aws_discovery.teleport_discovery_service_iam_role_arn }
```

`aws_matchers` object schema:

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `types` | list(string) | required | `["ec2"]`, `["eks"]`, or `["ec2","eks"]`. |
| `regions` | list(string) | `["*"]` | `["*"]` matches all regions. |
| `tags` | map(list(string)) | `{"*": ["*"]}` | Tag filter. |
| `kube_app_discovery` | bool | `null` | EKS only. Set `true` to enroll HTTP apps inside the cluster. Omit otherwise. |

Example, EC2 in all regions and EKS in us-east-1, each with its own tag filter:

```hcl
aws_matchers = [
  { types = ["ec2"], regions = ["*"],         tags = { env  = ["prod"] } },
  { types = ["eks"], regions = ["us-east-1"], tags = { team = ["platform"] }, kube_app_discovery = true },
]
```
