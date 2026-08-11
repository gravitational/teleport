output "ecs_cluster_name" {
  description = "Name of the ECS cluster for the Teleport ECS service."
  value       = module.teleport_db_service.ecs_cluster_name
}

output "ecs_cluster_arn" {
  description = "ARN of the ECS cluster for the Teleport ECS service."
  value       = module.teleport_db_service.ecs_cluster_arn
}

output "ecs_service_name" {
  description = "Name of the Teleport ECS service."
  value       = module.teleport_db_service.ecs_service_name
}

output "ecs_service_arn" {
  description = "ARN of the Teleport ECS service."
  value       = module.teleport_db_service.ecs_service_arn
}

output "ecs_task_definition_arn" {
  description = "ARN of the Teleport ECS task definition."
  value       = module.teleport_db_service.ecs_task_definition_arn
}

output "ecs_task_cloudwatch_log_group_name" {
  description = "Name of the CloudWatch log group for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_task_cloudwatch_log_group_name
}

output "ecs_task_cloudwatch_log_group_arn" {
  description = "ARN of the CloudWatch log group for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_task_cloudwatch_log_group_arn
}

output "security_group_id" {
  description = "Security group ID created for the Teleport ECS service."
  value       = module.teleport_db_service.security_group_id
}

output "ecs_execution_role_arn" {
  description = "The ARN of the execution IAM role for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_execution_role_arn
}

output "ecs_execution_role_name" {
  description = "The name of the execution IAM role for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_execution_role_name
}

output "ecs_task_role_arn" {
  description = "The ARN of the task IAM role for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_task_role_arn
}

output "ecs_task_role_name" {
  description = "The name of the task IAM role for the Teleport ECS task."
  value       = module.teleport_db_service.ecs_task_role_name
}

output "teleport_provision_token_allow_aws_arn" {
  description = "A value that can be used with a Teleport IAM join token to allow the ECS cluster to join the Teleport cluster using its IAM credentials."
  value       = module.teleport_db_service.teleport_provision_token_allow_aws_arn
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision token that allows the to join the cluster using AWS IAM credentials."
  value       = try(nonsensitive(teleport_provision_token.agent_aws_iam[0].metadata.name), null)
}

output "teleport_config" {
  description = "Teleport configuration used by the ECS task."
  value       = module.teleport_db_service.teleport_config
}
