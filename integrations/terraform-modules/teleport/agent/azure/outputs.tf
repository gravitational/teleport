output "container_group_ids" {
  description = "Resource IDs of the deployed agents, one per replica."
  value       = azurerm_container_group.teleport_agent[*].id
}

output "container_group_names" {
  description = "Names of the deployed agents, one per replica."
  value       = azurerm_container_group.teleport_agent[*].name
}

output "container_group_urls" {
  description = "Azure portal URLs for the deployment status and container logs of each deployed agent."
  value       = [for group in azurerm_container_group.teleport_agent : "https://portal.azure.com/#resource${group.id}/overview"]
}

output "log_analytics_workspace_id" {
  description = "Resource ID of the Log Analytics workspace that collects agent logs."
  value       = one(azurerm_log_analytics_workspace.this[*].id)
}

output "log_analytics_workspace_name" {
  description = "Name of the Log Analytics workspace that collects agent logs."
  value       = one(azurerm_log_analytics_workspace.this[*].name)
}

output "managed_identity" {
  description = "The user-assigned managed identity used by the deployed agents."
  value       = var.create ? local.managed_identity : null
}

output "container_logs_follow_command" {
  description = "The command to follow the logs of the deployed agents' containers."
  value       = [for group in azurerm_container_group.teleport_agent : "az container logs --follow --name ${group.name} --resource-group ${group.resource_group_name}"]
}