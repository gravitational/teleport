## Teleport AWS Organization Discovery Example

Configuration in this directory creates AWS and Teleport resources necessary for Teleport to discover resources in multiple AWS accounts under the same Organization.

Currently, only EC2 discovery is supported when doing organization-wide discovery.

Run Terraform with credentials from the AWS Organization management account or a delegated administrator account. The Discovery Service and Auth Service must use credentials from one of these accounts to call the required AWS Organizations APIs.

After applying, you have to manually create an IAM Role in each target account using the details provided in the `aws_child_account_iam_role_template` output, which you can get by running `terraform output`. When the root or `*` is included, this also includes the management or delegated administrator account.

When not using the AWS OIDC integration, you also have to create two extra IAM Roles in a management or delegated administrator account:

- `teleport_organization_account_enumeration_iam_role_template`: must be accessible from the Discovery Service and is used to enumerate all the accounts under the Organization and to assume the role created in each target account.
- `teleport_organization_join_validation_iam_role_template`: must be accessible from the Auth Service and is used to accept join attempts from target EC2 instances.

You must also set `aws_organization_discovery_iam_principal_arn` to the ARN of the AWS IAM principal used by the Discovery Service. The child account IAM role template uses this ARN in its trust policy.

You can get the role details by running `terraform output`.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| aws | >= 5.0 |
| teleport | >= 18.8.3 |
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
