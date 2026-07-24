## Teleport ECS Deployment

This module deploys a Teleport service to AWS ECS.

## Prerequisites
<!-- lint ignore absolute-docs-links -->
- [Configure Teleport Terraform Provider](https://goteleport.com/docs/configuration/terraform-provider/)
- [Configure AWS Terraform provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)

## Examples

Refer to the [examples](./examples) for example usage of this module.

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| aws | >= 6.0 |
| http | >= 3.0 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | >= 6.0 |
| http | >= 3.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [aws_cloudwatch_log_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_ecs_cluster.teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_cluster) | resource |
| [aws_ecs_service.teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_service) | resource |
| [aws_ecs_task_definition.teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_role.ecs_execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.ecs_task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.ecs_execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy.ecs_task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_security_group.teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_vpc_security_group_egress_rule.allow_all_outbound_from_teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_caller_identity.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_iam_policy_document.ecs_execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.ecs_execution_trust](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.ecs_task_trust](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |
| [aws_subnet.teleport_agent](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/subnet) | data source |
| [http_http.managed_updates](https://registry.terraform.io/providers/hashicorp/http/latest/docs/data-sources/http) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| apply\_aws\_tags | Additional AWS tags to apply to all created AWS resources. | `map(string)` | `{}` | no |
| assign\_public\_ip | Whether to assign public IP addresses to Teleport agent ECS tasks. If this is set to true, then var.ecs\_service\_subnets must be public subnets (route to an internet gateway). Otherwise, var.ecs\_service\_subnets must be private subnets (route to a NAT gateway). | `bool` | `false` | no |
| create | Toggle creation of all resources. | `bool` | `true` | no |
| create\_security\_group | Whether to create a security group for the Teleport agent ECS tasks. | `bool` | `true` | no |
| ecs\_cluster\_name | Name of the ECS cluster. | `string` | `"teleport"` | no |
| ecs\_service\_name | Name of the ECS service. | `string` | `"teleport-service"` | no |
| ecs\_service\_subnets | Subnet IDs where the Teleport agent will be deployed. If var.assign\_public\_ip is true, then all of these subnets must be public subnets (route to an internet gateway). If var.assign\_public\_ip is false, then all of these subnets must be private subnets (route to a NAT gateway). | `list(string)` | n/a | yes |
| ecs\_task\_cloudwatch\_log\_group\_name | Name for the ECS task CloudWatch log group. | `string` | `"ecs-teleport"` | no |
| ecs\_task\_cloudwatch\_log\_group\_region | AWS region for the ECS task CloudWatch log group. Defaults to the AWS provider region. | `string` | `null` | no |
| ecs\_task\_cloudwatch\_log\_group\_retention\_days | Number of days to retain logs in the ECS task CloudWatch log group. | `number` | `30` | no |
| ecs\_task\_cloudwatch\_log\_group\_skip\_destroy | Whether to preserve the ECS task CloudWatch log group when destroying module resources. Set to true if you do not wish the log group (and any logs it may contain) to be deleted at destroy time, and instead just remove the log group from the Terraform state. | `bool` | `false` | no |
| ecs\_task\_cpu | Number of cpu units used by the ECS task. | `string` | `"2048"` | no |
| ecs\_task\_desired\_count | Desired number of Teleport ECS tasks to run. | `number` | `2` | no |
| ecs\_task\_force\_new\_deployment | Set to true to force the ECS service to redeploy tasks without configuration changes. | `bool` | `false` | no |
| ecs\_task\_memory | Amount (in MiB) of memory used by the ECS task. | `string` | `"4096"` | no |
| ecs\_task\_name | Name of the ECS task. | `string` | `"teleport-agent"` | no |
| ecs\_task\_role\_inline\_policy | Optional JSON policy document to attach inline to the ECS task IAM role. | `string` | `null` | no |
| environment\_vars | Environment variables to set on the Teleport ECS container. | `map(string)` | `{}` | no |
| managed\_updates\_enabled | Whether to resolve the Teleport container version from the configured Managed Updates endpoint when applying this module. | `bool` | `false` | no |
| managed\_updates\_group | Update group to query through the v2 Managed Updates endpoint. | `string` | `"default"` | no |
| security\_group\_ids | Additional security group IDs to attach to the Teleport agent ECS tasks. | `list(string)` | `[]` | no |
| teleport\_config | Teleport configuration. Write the configuration using native Terraform syntax. Warning: sensitive data, such as static join tokens, is visible to anyone who can read the task definition. | `any` | n/a | yes |
| teleport\_container\_image | Container image used for Teleport ECS tasks. | `string` | `"public.ecr.aws/gravitational/teleport-ent-distroless"` | no |
| teleport\_version | The version of Teleport to deploy. Generally, the version of Teleport should be controlled by using the appropriate version of this module. This variable is intended for development usage. | `string` | `"18.10.0"` | no |
| vpc\_id | VPC ID where the Teleport agent will be deployed. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| ecs\_execution\_role\_arn | The ARN of the execution IAM role for the Teleport ECS task. |
| ecs\_execution\_role\_name | The name of the execution IAM role for the Teleport ECS task. |
| ecs\_task\_role\_arn | The ARN of the task IAM role for the Teleport agent ECS task. |
| ecs\_task\_role\_name | The name of the task IAM role for the Teleport agent ECS task. |
| security\_group\_id | Security group ID created for the Teleport agent ECS service. |
| teleport\_provision\_token\_allow\_aws\_arn | A value that can be used with a Teleport IAM join token to allow the ECS cluster to join the Teleport cluster using its IAM credentials. |
<!-- END_TF_DOCS -->
