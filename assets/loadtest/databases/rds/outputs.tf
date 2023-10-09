output "database_agent_role_arn" {
  description = "IAM Role ARN that must be used by the database agents."
  value       = module.iam_eks_role.iam_role_arn
}
