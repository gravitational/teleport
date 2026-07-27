output "security_group_id" {
  description = "Security group ID created for the Teleport agent ECS service."
  value       = one(aws_security_group.teleport_agent[*].id)
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
  description = "The ARN of the task IAM role for the Teleport agent ECS task."
  value       = one(aws_iam_role.ecs_task[*].arn)
}

output "ecs_task_role_name" {
  description = "The name of the task IAM role for the Teleport agent ECS task."
  value       = one(aws_iam_role.ecs_task[*].name)
}

output "teleport_provision_token_allow_aws_arn" {
  description = <<EOF
A value that can be used with a Teleport IAM join token to allow the ECS cluster to join the Teleport cluster using its IAM credentials.
EOF
  value = (
    var.create
    ? format(
      "arn:%s:sts::%s:assumed-role/%s/*",
      one(data.aws_partition.this[*].partition),
      one(data.aws_caller_identity.this[*].account_id),
      one(aws_iam_role.ecs_task[*].name),
    )
    : null
  )
}
