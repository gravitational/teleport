output "psql_fqdn" {
  value = azurerm_postgresql_flexible_server.pgbk.fqdn
}

output "psql_adminuser" {
  value = azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_adminuser.principal_name
}

output "rg_name" {
  value = azurerm_kubernetes_cluster.kube_cluster.resource_group_name
}

output "aks_name" {
  value = azurerm_kubernetes_cluster.kube_cluster.name
}

output "monitoring_ns" {
  value = helm_release.monitoring.namespace
}

output "monitoring_release" {
  value = helm_release.monitoring.name
}

output "teleport_ns" {
  value = local.teleport_namespace
}

output "teleport_release" {
  value = local.teleport_release
}

output "public_addr" {
  value = "https://${trimsuffix(azurerm_dns_a_record.proxy.fqdn, ".")}/"
}
