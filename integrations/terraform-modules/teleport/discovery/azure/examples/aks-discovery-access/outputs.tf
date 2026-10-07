output "azure_discovery" {
  description = "Discovery configuration, OIDC integration, and Azure discovery identity."
  value       = module.azure_discovery
}

output "azure_agent" {
  description = "Kubernetes Service identity, container deployments, and commands to follow their logs."
  value       = module.azure_agent
}
