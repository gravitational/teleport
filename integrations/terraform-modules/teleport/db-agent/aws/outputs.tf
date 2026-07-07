output "security_group_id" {
  description = "Security group ID created for the Teleport db agent ECS service."
  value       = module.teleport_db_service.security_group_id
}

output "ecs_execution_role_arn" {
  description = "The ARN of the execution IAM role for the Teleport db agent ECS task."
  value       = module.teleport_db_service.ecs_execution_role_arn
}

output "ecs_execution_role_name" {
  description = "The name of the execution IAM role for the Teleport db agent ECS task."
  value       = module.teleport_db_service.ecs_execution_role_name
}

output "ecs_task_role_arn" {
  description = "The ARN of the task IAM role for the Teleport db agent ECS task."
  value       = module.teleport_db_service.ecs_task_role_arn
}

output "ecs_task_role_name" {
  description = "The name of the task IAM role for the Teleport db agent ECS task."
  value       = module.teleport_db_service.ecs_task_role_name
}

output "teleport_provision_token_allow_aws_arn" {
  description = "A value that can be used with a Teleport IAM join token to allow the ECS cluster to join the Teleport cluster using its IAM credentials."
  value       = module.teleport_db_service.teleport_provision_token_allow_aws_arn
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision token that allows the db agent to join the cluster using AWS IAM credentials."
  value       = nonsensitive(try(teleport_provision_token.agent_aws_iam[0].metadata.name, null))
}
