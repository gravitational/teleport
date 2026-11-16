output "azure_discovery" {
  description = "AKS discovery configuration, OIDC integration, and Azure discovery identity."
  value       = module.azure_discovery
}

output "azure_agent" {
  description = "Public Kubernetes Service identity, replicas, and log commands."
  value       = module.azure_agent
}

output "azure_agent_private" {
  description = "Private Kubernetes Service identity, replicas, and log commands."
  value       = module.azure_agent_private
}
