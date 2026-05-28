# Teleport AWS Organization Discovery Example

Configuration in this directory creates AWS and Teleport resources necessary for Teleport to discover resources in multiple AWS accounts under the same Organization.

Currently, only EC2 discovery is supported when doing organization-wide discovery.

After applying, you have to manually create an IAM Role in each children account using the details provided in the `aws_child_account_iam_role_template` output, which you can get by running `terraform output`.

When not using the AWS OIDC integration, you also have to create two extra IAM Roles in the management account:
- `teleport_organization_account_enumeration_iam_role_template`: must be accessible from the Discovery Service and is used to enumerate all the accounts under the Organization and to assume the role created in each one
- `teleport_organization_join_validation_iam_role_template`: must be accessible from the Auth Service and is used to accept join attempts from any of the child account EC2 instances.

You can get the role details by running `terraform output`.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.0 |
| aws | >= 5.0 |
| teleport | >= 18.8.1 |
| tls | >= 4.0 |

## Providers

No providers.

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| aws\_discovery | ../.. | n/a |

## Resources

No resources.

## Inputs

No inputs.

## Outputs

| Name | Description |
| ---- | ----------- |
| aws\_discovery | n/a |
<!-- END_TF_DOCS -->