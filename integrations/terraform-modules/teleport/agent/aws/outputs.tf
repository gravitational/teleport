output "ecs_cluster_name" {
  description = "Name of the ECS cluster for the Teleport ECS service."
  value       = one(aws_ecs_cluster.teleport_agent[*].name)
}

output "ecs_cluster_arn" {
  description = "ARN of the ECS cluster for the Teleport ECS service."
  value       = one(aws_ecs_cluster.teleport_agent[*].arn)
}

output "ecs_service_name" {
  description = "Name of the Teleport ECS service."
  value       = one(aws_ecs_service.teleport_agent[*].name)
}

output "ecs_service_arn" {
  description = "ARN of the Teleport ECS service."
  value       = one(aws_ecs_service.teleport_agent[*].id)
}

output "ecs_service_subnet_ids" {
  description = "Subnet IDs selected for the Teleport ECS service, up to the AWS limit of 16. Private subnets are preferred over public subnets."
  value       = local.ecs_service_subnets
}

output "assign_public_ip" {
  description = "Whether the Teleport ECS tasks are assigned public IP addresses."
  value       = local.assign_public_ip
}

output "private_subnet_ids" {
  description = "Private subnet IDs provided by the caller or discovered from default routes through AWS NAT gateways."
  value       = local.private_subnet_ids
}

output "public_subnet_ids" {
  description = "Public subnet IDs provided by the caller or discovered from default routes through AWS internet gateways."
  value       = local.public_subnet_ids
}

output "intra_subnet_ids" {
  description = "Subnet IDs discovered without a default route through an AWS NAT gateway or internet gateway. Empty when subnet classification is provided by the caller."
  value       = local.intra_subnet_ids
}

output "ecs_task_definition_arn" {
  description = "ARN of the Teleport ECS task definition."
  value       = one(aws_ecs_task_definition.teleport_agent[*].arn)
}

output "ecs_task_cloudwatch_log_group_name" {
  description = "Name of the CloudWatch log group for the Teleport ECS task."
  value       = one(aws_cloudwatch_log_group.this[*].name)
}

output "ecs_task_cloudwatch_log_group_arn" {
  description = "ARN of the CloudWatch log group for the Teleport ECS task."
  value       = one(aws_cloudwatch_log_group.this[*].arn)
}

output "security_group_id" {
  description = "Security group ID created for the Teleport ECS service. Null when create_security_group is false."
  value       = one(aws_security_group.teleport_agent[*].id)
}

output "database_security_group_id" {
  description = "Shared security group ID to attach to databases to allow ingress from the Teleport Database Service. Null when create_security_group is false."
  value       = one(aws_security_group.teleport_database_access[*].id)
}

output "ecs_execution_role_arn" {
  description = "The ARN of the execution IAM role for the Teleport ECS task."
  value       = one(aws_iam_role.ecs_execution[*].arn)
}

output "ecs_execution_role_name" {
  description = "The name of the execution IAM role for the Teleport ECS task."
  value       = one(aws_iam_role.ecs_execution[*].name)
}

output "ecs_task_role_arn" {
  description = "The ARN of the task IAM role for the Teleport ECS task."
  value       = one(aws_iam_role.ecs_task[*].arn)
}

output "ecs_task_role_name" {
  description = "The name of the task IAM role for the Teleport ECS task."
  value       = one(aws_iam_role.ecs_task[*].name)
}

output "teleport_provision_token_allow_aws_arn" {
  description = <<EOF
A value that can be used with a Teleport IAM join token to allow the ECS cluster to join the Teleport cluster using its IAM credentials.
EOF
  value       = local.teleport_provision_token_allow_aws_arn
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision token that allows the Teleport instance to join the cluster using AWS IAM credentials."
  value       = try(nonsensitive(teleport_provision_token.agent_aws_iam[0].metadata.name), null)
}
