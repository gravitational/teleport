output "psql_fqdn" {
  value = azurerm_postgresql_flexible_server.pgbk.fqdn
}

output "psql_adminuser" {
  value = azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_adminuser.principal_name
}

output "resource_group" {
  value = azurerm_resource_group.rg.name
}

output "aks_name" {
  value = azurerm_kubernetes_cluster.kube_cluster.name
}

output "monitoring_namespace" {
  value = helm_release.monitoring.namespace
}

output "monitoring_release" {
  value = helm_release.monitoring.name
}

output "teleport_namespace" {
  value = var.deploy_teleport ? helm_release.teleport.0.namespace : null
}

output "teleport_release" {
  value = var.deploy_teleport ? helm_release.teleport.0.name : null
}

output "public_addr" {
  value = "https://${trimsuffix(azurerm_dns_a_record.proxy.fqdn, ".")}/"
}
