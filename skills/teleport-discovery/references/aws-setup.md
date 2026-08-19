# AWS Discovery Setup

Version gate: if `cluster_version` is below `18.8.0`, stop: "AWS discovery with Terraform
requires Teleport 18.8.0 or later. This cluster is v`<cluster_version>`."

Resolve the common fields from the skill's Setup section, then these AWS fields.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `services` | none | `ec2` and `eks` matchers |
| `regions` | none | Ask, with `["*"]` as the default |
| `tags` | none | Ask, with `{"*": ["*"]}` as the default |
| `kube_app_discovery` | none | Omit from the plan |
| `existing_oidc_provider` | see **Existing OIDC provider** below | `no` |

## Existing OIDC provider

Let `<host>` be `proxy_addr` without its port. Resolve `existing_oidc_provider`:

1. Run `aws iam list-open-id-connect-providers`.
2. Find the provider whose ARN ends in `oidc-provider/<host>`. If none, set `no` and stop.
3. Run `aws iam get-open-id-connect-provider --open-id-connect-provider-arn <arn>` for that ARN only.
4. Strip the scheme and port from `.Url`. If it does not equal `<host>`, set `no` and stop.
5. If `.ClientIDList` includes `discover.teleport`, set `yes`. Otherwise add it with
   `aws iam add-client-id-to-open-id-connect-provider --open-id-connect-provider-arn <arn> --client-id discover.teleport`,
   then set `yes`.

## Write the Terraform

Let `<major>` be `cluster_version`'s major version. Set `<provider_version>` to
`>= 18.8.0, < 19.0.0` when `<major>` is 18, else `~> <major>.0`.

Declare the provider requirements and configuration:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "<provider_version>"
    }
  }
}

provider "teleport" {
  addr = "<proxy_addr>"
}
```

Write the discovery module:

```hcl
module "aws_discovery" {
  source  = "terraform.releases.teleport.dev/teleport/discovery/aws"
  version = "~> <major>.0"

  teleport_proxy_public_addr    = "<proxy_addr>"
  teleport_discovery_group_name = "<discovery_group>"

  aws_matchers = [
    # one object per matcher, per the schema below
  ]

  # Set to false only when existing_oidc_provider is yes:
  # create_aws_iam_openid_connect_provider = false
}

output "integration_name" {
  value = module.aws_discovery.teleport_integration_name
}

output "discovery_config_name" {
  value = module.aws_discovery.teleport_discovery_config_name
}

output "oidc_provider_arn" {
  value = module.aws_discovery.aws_oidc_provider_arn
}

output "discovery_role_arn" {
  value = module.aws_discovery.teleport_discovery_service_iam_role_arn
}
```

`aws_matchers` object schema:

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `types` | list(string) | required | `["ec2"]`, `["eks"]`, or `["ec2","eks"]`. |
| `regions` | list(string) | `["*"]` | `["*"]` matches all regions. |
| `tags` | map(list(string)) | `{"*": ["*"]}` | Tag filter. |
| `kube_app_discovery` | bool | `null` | EKS only. Set `true` to enroll HTTP apps inside the cluster. Omit otherwise. |

Omit any field whose value equals its default above.

Example, EC2 in all regions and EKS in us-east-1, each with its own tag filter:

```hcl
aws_matchers = [
  { types = ["ec2"], tags = { env = ["prod"] } },
  { types = ["eks"], regions = ["us-east-1"], tags = { team = ["platform"] }, kube_app_discovery = true },
]
```
