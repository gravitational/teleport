output "postgres_address" {
  description = "PostgreSQL address including port"
  value       = try("${module.pg.db_instance_address}:${var.postgres_port}", "")
}

output "mysql_address" {
  description = "MySQL address including port"
  value       = try("${module.mysql.db_instance_address}:${var.mysql_port}", "")
}

output "database_agent_role_arn" {
  description = "IAM Role ARN that must be used by the database agents."
  value       = module.iam_eks_role.iam_role_arn
}
