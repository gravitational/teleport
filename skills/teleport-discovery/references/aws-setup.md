# AWS Discovery Setup

Version gate: if `cluster_version` is below `18.8.0`, stop: "AWS discovery with Terraform
requires Teleport 18.8.0 or later. This cluster is v`<cluster_version>`."

Resolve the common fields from the skill's Setup section, then these AWS fields.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `scope` | see **Scope** below | `single account` |
| `services` | none | Organization scope: `ec2`; single-account scope: `ec2` and `eks` matchers |
| `regions` | none | Ask, with `["*"]` as the default |
| `tags` | none | Ask, with `{"*": ["*"]}` as the default |
| `kube_app_discovery` | none | Omit from the plan |
| `existing_oidc_provider` | see **Existing OIDC provider** below | `no` |
| `organizational_units_include` | none | Only when scope is Organization: Ask, and default to everything: `["*"]` |
| `organizational_units_exclude` | none | Only when scope is Organization: Ask but default to empty `[]` |
| `organization_credentials_confirmed` | none | Only when scope is Organization: Ask |

## Existing OIDC provider

Let `<host>` be `proxy_addr` without its port. Resolve `existing_oidc_provider`:

1. Run `aws iam list-open-id-connect-providers`.
2. Find the provider whose ARN ends in `oidc-provider/<host>`. If none, set `no` and stop.
3. Run `aws iam get-open-id-connect-provider --open-id-connect-provider-arn <arn>` for that ARN only.
4. Strip the scheme and port from `.Url`. If it does not equal `<host>`, set `no` and stop.
5. If `.ClientIDList` includes `discover.teleport`, set `yes`. Otherwise add it with
   `aws iam add-client-id-to-open-id-connect-provider --open-id-connect-provider-arn <arn> --client-id discover.teleport`,
   then set `yes`.

## Scope

Users can enroll a single account or an entire AWS Organization.

Resolve `scope` before `services`.
Set it to Organization when the request names an AWS Organization or requests discovery across multiple, all, or every account.
Otherwise, use single-account scope.

For AWS Organization discovery:

1. If `cluster_version` is below `18.8.3`, stop: "AWS Organization discovery requires Teleport 18.8.3 or later. This cluster is v`<cluster_version>`."
1. Only accept `ec2` as services to enroll. If the request includes another service, stop and explain that organization-wide discovery currently supports only EC2.
1. Ask which organizational unit IDs to include through `organizational_units_include`. Accept `*` only when the user explicitly chooses all accounts; otherwise collect one or more Organization Root IDs or Organizational Unit IDs. Never infer `*` from an omitted or unusable answer.
1. Resolve `organizational_units_exclude` to `[]` when the user explicitly requests all or every account without naming exclusions. Otherwise ask which exact Root or Organizational Unit IDs to exclude. An empty list means no exclusions.
1. Validate the filters before writing: `include` must be nonempty; `*` must be its only entry; every other value must be a Root ID (`r-...`) or Organizational Unit ID (`ou-...`); and `exclude` must contain only exact Root or Organizational Unit IDs, never `*`.
1. Ask the user to confirm that Terraform will run with credentials from the AWS Organization management account or a delegated administrator account and that the Discovery Service and Auth Service use credentials from one of those accounts. Set `organization_credentials_confirmed` only on explicit confirmation. Setup may write the Terraform without confirmation, but Apply must stop until it is confirmed.

An IAM role must also be created manually in every included account. When `*` or the root is included, this includes the management or delegated administrator account.
The `aws_child_account_iam_role_template` Terraform output contains the required role name, trust policy, and permissions.
Apply must pause for these roles as described in `references/apply.md`.

## Write the Terraform

Let `<major>` be `cluster_version`'s major version.
For Teleport 18, set `<provider_version>` to `>= 18.8.3, < 19.0.0` for AWS Organization discovery and `>= 18.8.0, < 19.0.0` otherwise.
For other majors, set it to `~> <major>.0`.

Set `<module_version>` to `>= 18.10.2, < 19.0.0` for AWS Organization discovery on Teleport 18.
Otherwise, set it to `~> <major>.0`.

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
  version = "<module_version>"

  teleport_proxy_public_addr    = "<proxy_addr>"
  teleport_discovery_group_name = "<discovery_group>"

  aws_matchers = [
    # one object per matcher, per the schema below
  ]

  # Fill this section only when using organization-wide discovery.
  # aws_organization_discovery = {
  #   organizational_units = {
  #     include = <organizational_units_include>
  #     exclude = <organizational_units_exclude>
  #   }
  # }

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

For AWS Organization discovery, also write:

```hcl
output "aws_child_account_iam_role_template" {
  value = module.aws_discovery.aws_child_account_iam_role_template
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
